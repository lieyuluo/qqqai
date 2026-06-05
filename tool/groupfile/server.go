package groupfile

import (
	"encoding/json"
	"log"
	"net/http"
)

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/index/group-file", indexGroupFileHandler)
}

func indexGroupFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, IndexResponse{Message: "解析索引请求失败: " + err.Error()})
		return
	}

	result, err := Index(r.Context(), req)
	if err != nil {
		log.Printf("群文件索引失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, IndexResponse{
			FileName: req.FileName,
			Message:  ReplyForError(req, err),
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("写入 JSON 响应失败: %v", err)
	}
}
