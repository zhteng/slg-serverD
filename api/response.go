package api

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// writeJSON 将给定数据包装为统一格式并写入 ResponseWriter
func writeJSON(w http.ResponseWriter, httpStatus int, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

// success 快捷成功返回（HTTP 200, code 1, msg "success"）
func success(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, 1, "success", data)
}

func fail(w http.ResponseWriter, httpStatus int, msg string) {
	writeJSON(w, httpStatus, 0, msg, nil)
}
