package main

import "net/http"

type (
	Host struct {
		Hostname string
	}
)

func (h *Host) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
	case "POST":
	case "PUT":
	case "DELETE":
		h.De
	}
}

func (h *Host) Index() {

}

func (h *Host) Show(id string) {

}

func (h *Host) Create(params *ControllerParams) {

}

func (h *Host) Update(id string, params *ControllerParams) {

}

func (h *Host) Delete(id string) {

}
