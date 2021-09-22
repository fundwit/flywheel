package es

import (
	"bytes"
	"encoding/json"
	"flywheel/bizerror"
	"flywheel/session"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/estransport"
	"github.com/fundwit/go-commons/types"
	"github.com/sirupsen/logrus"
)

var (
	SearchFunc             = Search
	IndexFunc              = Index
	GetDocumentFunc        = GetDocument
	DropIndexFunc          = DropIndex
	DeleteDocumentByIdFunc = DeleteDocumentById
)

type H map[string]interface{}

type ESGetResult struct {
	Index string `json:"_index"`
	Type  string `json:"_type"`
	Id    string `json:"_id"`

	Version     int `json:"_version"`
	SeqNO       int `json:"_seq_no"`
	PrimaryTerm int `json:"_primary_term"`

	Found  bool   `json:"found"`
	Source Source `json:"_source"`
}

const (
	DeleteResultDeleted  = "deleted"
	DeleteResultNotFound = "not_found"
)

type ESDeleteResult struct {
	Index string `json:"_index"`
	Type  string `json:"_type"`
	Id    string `json:"_id"`

	Version     int `json:"_version"`
	SeqNO       int `json:"_seq_no"`
	PrimaryTerm int `json:"_primary_term"`

	Result string         `json:"result"` // deleted, not_found
	Shards ESSearchShards `json:"_shards"`
}

type ESSearchResult struct {
	Took    int            `json:"took"`
	TimeOut bool           `json:"timed_out"`
	Shards  ESSearchShards `json:"_shards"`
	Hits    ESSearchHits   `json:"hits"`
}
type ESSearchShards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}
type ESSearchHits struct {
	Total    ESSearchHitsTotal `json:"total"`
	MaxScore float64           `json:"max_score"`
	Hits     []ESSearchHit     `json:"hits"`
}
type ESSearchHitsTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}
type ESSearchHit struct {
	Index string `json:"_index"`
	Type  string `json:"_type"`
	Id    string `json:"_id"`

	Score  float64       `json:"_score"`
	Source Source        `json:"_source"`
	Sort   []interface{} `sort:"sort"`
}

type Source string

func (d *Source) UnmarshalJSON(data []byte) (err error) {
	*d = Source(data)
	return
}

func (d *Source) MarshalJSON() ([]byte, error) {
	return []byte(*d), nil
}

// ELASTICSEARCH_URL
var ActiveESClient *elasticsearch.Client

// CreateClientFromEnv ELASTICSEARCH_URL
func CreateClientFromEnv() *elasticsearch.Client {
	debug := os.Getenv("GIN_MODE") == "debug"
	conf := elasticsearch.Config{
		Logger:    &estransport.TextLogger{Output: os.Stdout, EnableRequestBody: debug, EnableResponseBody: debug},
		Transport: &TracingTransport{Transport: http.DefaultTransport},
	}
	client, err := elasticsearch.NewClient(conf)
	if err != nil {
		panic(err)
	}

	ActiveESClient = client
	return client
}

func DropIndex(index string, s *session.Session) error {
	req := esapi.IndicesDeleteRequest{
		Index: []string{index},
	}

	res, err := req.Do(s.Context, ActiveESClient)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error response status %s", res.Status())
	} else {
		logrus.Debugln(res.String())
	}
	return nil
}

func Index(index string, id types.ID, doc interface{}, s *session.Session) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id.String(),
		Body:       bytes.NewReader(buf.Bytes()),
		Refresh:    "true",
	}

	logrus.Debugln("saved document body:", buf.String())
	res, err := req.Do(s.Context, ActiveESClient)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error response status %s", res.Status())
	} else {
		logrus.Debugln(res.String())
	}
	return nil
}

func Search(index string, query interface{}, s *session.Session) (*ESSearchResult, error) {
	// "query": { "match": {"title": "test"}}
	var q bytes.Buffer
	if err := json.NewEncoder(&q).Encode(query); err != nil {
		return nil, err
	}

	res, err := ActiveESClient.Search(
		ActiveESClient.Search.WithContext(s.Context),
		ActiveESClient.Search.WithIndex(index),
		ActiveESClient.Search.WithBody(&q),
		ActiveESClient.Search.WithTrackTotalHits(true),
		ActiveESClient.Search.WithPretty(),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf(res.String())
	}

	r := ESSearchResult{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf(res.String())
	}
	return &r, nil
}

func GetDocument(index string, id types.ID, s *session.Session) (Source, error) {
	res, err := ActiveESClient.Get(index, id.String(), ActiveESClient.Get.WithContext(s.Context))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.IsError() {
		return "", fmt.Errorf("error response status %s", res.Status())
	}
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	logrus.Debugln("get document body: ", string(bytes))
	result := ESGetResult{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return "", err
	}
	if !result.Found {
		return "", bizerror.ErrNotFound
	}
	return result.Source, nil
}

func DeleteDocumentById(index string, id types.ID, s *session.Session) error {
	res, err := ActiveESClient.Delete(index, id.String(),
		ActiveESClient.Delete.WithRefresh("true"),
		ActiveESClient.Delete.WithContext(s.Context))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	logrus.Debugln("delete document respone body: ", string(bytes))
	result := ESDeleteResult{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	if result.Result == DeleteResultDeleted || result.Result == DeleteResultNotFound {
		return nil
	}
	return fmt.Errorf("delete error on elasticsearch: %v", string(bytes))
}
