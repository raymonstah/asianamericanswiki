package server

import "net/http"

type ErrorResponse struct {
	Status int
	Err    error
}

func (e ErrorResponse) Error() string {
	return e.Err.Error()
}

func NewInternalServerError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusInternalServerError,
		Err:    err,
	}
}

func NewTooManyRequestsError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusTooManyRequests,
		Err:    err,
	}
}

func NewUnauthorizedError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusUnauthorized,
		Err:    err,
	}
}

func NewForbiddenError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusForbidden,
		Err:    err,
	}
}

func NewBadRequestError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusBadRequest,
		Err:    err,
	}
}

func NewNotFoundError(err error) ErrorResponse {
	return ErrorResponse{
		Status: http.StatusNotFound,
		Err:    err,
	}
}
