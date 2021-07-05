package misc

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type PagedBody struct {
	List  interface{} `json:"data"`
	Total uint64      `json:"total"`
}
