/**
* Generated by go-doudou v2.0.4.
* Don't edit!
 */
package httpsrv

import (
	"net/http"

	"github.com/unionj-cloud/go-doudou/v2/framework"
	"github.com/unionj-cloud/go-doudou/v2/framework/rest"
)

type PromHandler interface {
	Query(w http.ResponseWriter, r *http.Request)
	GetQuery(w http.ResponseWriter, r *http.Request)
	Query_range(w http.ResponseWriter, r *http.Request)
	GetQuery_range(w http.ResponseWriter, r *http.Request)
	GetLabel_Label_nameValues(w http.ResponseWriter, r *http.Request)
}

func Routes(handler PromHandler) []rest.Route {
	return []rest.Route{
		{
			Name:        "Query",
			Method:      "POST",
			Pattern:     "/query",
			HandlerFunc: handler.Query,
		},
		{
			Name:        "GetQuery",
			Method:      "GET",
			Pattern:     "/query",
			HandlerFunc: handler.GetQuery,
		},
		{
			Name:        "Query_range",
			Method:      "POST",
			Pattern:     "/query_range",
			HandlerFunc: handler.Query_range,
		},
		{
			Name:        "GetQuery_range",
			Method:      "GET",
			Pattern:     "/query_range",
			HandlerFunc: handler.GetQuery_range,
		},
		{
			Name:        "GetLabel_Label_nameValues",
			Method:      "GET",
			Pattern:     "/label/:label_name/values",
			HandlerFunc: handler.GetLabel_Label_nameValues,
		},
	}
}

var RouteAnnotationStore = framework.AnnotationStore{}
