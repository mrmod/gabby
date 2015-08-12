package main

import (
	"net/http"
)

func newControllerError(code int, err error, desc string) *ControllerError {
	return &ControllerError{code, err, desc}
}

func newControllerParams(request http.Request) *ControllerParams {
	if err := request.ParseForm(); err != nil {
		errors := []*ControllerError{
			newControllerError(http.StatusBadRequest, err, "Bad params"),
		}
		return &ControllerParams{errors}
	}

	return &ControllerParams{
		[]*ControllerParams{},
		request.Form,
	}
}

func (p *ControllerParams) HasErrors() bool {
	return len(p.Errors) > 0
}
