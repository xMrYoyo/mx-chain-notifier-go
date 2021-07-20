package handlers

import "github.com/gin-gonic/gin"

type EndpointHandler struct {
	Path        string
	Method      string
	HandlerFunc gin.HandlerFunc
}

type groupHandler struct {
	endpointHandlersMap map[string][]EndpointHandler
}

// NewGroupHandler creates an instance of a groupHandler
func NewGroupHandler() *groupHandler {
	return &groupHandler{
		endpointHandlersMap: make(map[string][]EndpointHandler),
	}
}

// RegisterEndpoints registers the endpoints groups and the corresponding handlers
// It should be called after all the handlers have been defined
func (g *groupHandler) RegisterEndpoints(r *gin.Engine) {
	for route, handlersGroup := range g.endpointHandlersMap {
		routerGroup := r.Group(route)
		{
			for _, h := range handlersGroup {
				routerGroup.Handle(h.Method, h.Path, h.HandlerFunc)
			}
		}
	}
}

// AddEndpointHandlers inserts a list of endpoint handlers to the map
// The key of the endpointHandlersMap is the base path of the group
// The method is not thread-safe and does not validate inputs
func (g *groupHandler) AddEndpointHandlers(route string, handlers []EndpointHandler) {
	g.endpointHandlersMap[route] = handlers
}

type apiResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// JsonResponse is a wrapper for gin.Context JSON payload
func JsonResponse(c *gin.Context, status int, data interface{}, error string) {
	c.JSON(status, apiResponse{
		Data:  data,
		Error: error,
	})
}