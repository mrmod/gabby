package main

import (
	"net/http"
	"net/url"
	"strings"
)

type (
	// Controller is an interface for REST-like
	// services
	Controller interface {
		// /api/v1/resourceName
		// Absolute path of the resource name
		ResourcePath() string
		// GET / Index
		Index() []Resource
		// GET /:id
		Show(string) Resource
		// POST /
		Create(*ControllerParams)
		// PUT /:id
		Update(string, *ControllerParams)
		// DELETE /:id
		Delete(string)
	}
	ControllerParams struct {
		Errors []*ControllerError
		url.Values
	}
	ControllerError struct {
		StatusCode  int
		Error       error
		Description string
	}
	Resource interface {
		// MustJson() []byte
		Json() []byte
		// MustHtml() []byte
		Html() []byte
	}
)

func resourceId(controller Controller, request *http.Request) string {
	strings.Split(request.RequestURI)
}

func main() {

}
