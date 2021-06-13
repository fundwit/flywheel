package workcontribution

import (
	"flywheel/security"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	PathWorkContributionsRoot      = "/v1/contributions"
	PathWorkContributionsQueryRoot = "/v1/contributor-queries"
)

var (
	QueryWorkContributionsFunc = QueryWorkContributions
	BeginWorkContributionFunc  = BeginWorkContribution
	FinishWorkContributionFunc = FinishWorkContribution
)

func RegisterWorkContributionsHandlers(r *gin.Engine, middleWares ...gin.HandlerFunc) {
	g := r.Group(PathWorkContributionsRoot, middleWares...)
	g.POST("", HandleBeginContribution)
	g.PUT("", HandleFinishContribution)

	q := r.Group(PathWorkContributionsQueryRoot, middleWares...)
	q.POST("", HandleQueryContributions)
}

func HandleQueryContributions(c *gin.Context) {
	query := WorkContributionsQuery{}
	if err := c.MustBindWith(&query, binding.Query); err != nil {
		panic(err)
	}
	body := WorkContributionsQuery{}
	if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
		panic(err)
	}
	query.WorkKeys = append(query.WorkKeys, body.WorkKeys...)

	results, err := QueryWorkContributionsFunc(query, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, results)
}

func HandleBeginContribution(c *gin.Context) {
	body := WorkContribution{}
	if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
		panic(err)
	}

	detail, err := BeginWorkContributionFunc(&body, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, gin.H{"id": &detail})
}

func HandleFinishContribution(c *gin.Context) {
	body := WorkContribuitonFinishBody{}
	if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
		panic(err)
	}

	err := FinishWorkContributionFunc(&body, security.FindSecurityContext(c))
	if err != nil {
		panic(err)
	}
	c.Status(http.StatusOK)
}
