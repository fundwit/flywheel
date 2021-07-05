package es

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/fundwit/go-commons/types"
)

// ELASTICSEARCH_URL
var ActiveESClient *elasticsearch.Client

func init() {
	client, err := elasticsearch.NewDefaultClient()
	if err != nil {
		panic(err)
	}
	ActiveESClient = client
}

// func Index(index string, doc interface{}) error {
// 	res, err := ActiveESClient.Index(index, &buf, elasticsearch.Index.WithDocumentType("doc"))
// 	if err != nil {
// 		failOnError(err, "Error Index response")
// 	}
// 	defer res.Body.Close()
// 	fmt.Println(res.String())
// }

func Index(index string, id types.ID, doc interface{}) error {
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

	res, err := req.Do(context.Background(), ActiveESClient)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[%s] Error indexing", res.Status())
	} else {
		// Deserialize the response into a map.
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			log.Printf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and indexed document version.
			log.Printf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
		}
	}
	return nil
}

func Search(doc interface{}) error {
	// "query": { "match": {"title": "test"}}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}

	res, err := ActiveESClient.Search(
		ActiveESClient.Search.WithContext(context.Background()),
		ActiveESClient.Search.WithIndex("test"),
		ActiveESClient.Search.WithBody(&buf),
		ActiveESClient.Search.WithTrackTotalHits(true),
		ActiveESClient.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			log.Fatalf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			log.Fatalf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
	}

	r := map[string]interface{}{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	// Print the response status, number of results, and request duration.

	log.Printf(
		"[%s] %d hits; took: %dms",
		res.Status(),
		int(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)),
		int(r["took"].(float64)),
	)
	// Print the ID and document source for each hit.
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		log.Printf(" * ID=%s, %s", hit.(map[string]interface{})["_id"], hit.(map[string]interface{})["_source"])
	}

	log.Println(strings.Repeat("=", 37))
	return nil
}

func GetDocument(index string, id types.ID) ([]byte, error) {
	res, err := ActiveESClient.Get(index, id.String())
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func DeleteByQuery() {

}
func DeleteById(index string, id types.ID) ([]byte, error) {
	res, err := ActiveESClient.Delete(index, id.String())
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func UpdateById() {

}

func UpdateByQuery() {

}
