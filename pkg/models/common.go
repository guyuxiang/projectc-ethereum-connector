package models

import "encoding/json"

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Failure(code int, message string) Response {
	return Response{Code: code, Message: message, Data: nil}
}

type RawNumber string

func (n RawNumber) MarshalJSON() ([]byte, error) {
	if n == "" {
		return []byte("0"), nil
	}
	return []byte(n), nil
}

type PageRequest[T any] struct {
	PageNo   int `json:"pageNo"`
	PageSize int `json:"pageSize"`
	Filter   T   `json:"filter"`
}

type PageResponse[T any] struct {
	PageNo     int `json:"pageNo"`
	PageSize   int `json:"pageSize"`
	TotalCount int `json:"totalCount"`
	Records    []T `json:"records"`
}

type JSONMap map[string]interface{}

func (m JSONMap) Clone() JSONMap {
	if m == nil {
		return JSONMap{}
	}
	raw, _ := json.Marshal(m)
	var out JSONMap
	_ = json.Unmarshal(raw, &out)
	return out
}
