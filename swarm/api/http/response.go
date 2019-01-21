
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/api"
)

var (
	htmlCounter      = metrics.NewRegisteredCounter("api.http.errorpage.html.count", nil)
	jsonCounter      = metrics.NewRegisteredCounter("api.http.errorpage.json.count", nil)
	plaintextCounter = metrics.NewRegisteredCounter("api.http.errorpage.plaintext.count", nil)
)

type ResponseParams struct {
	Msg       template.HTML
	Code      int
	Timestamp string
	template  *template.Template
	Details   template.HTML
}

//
//
//
//
//
//
func ShowMultipleChoices(w http.ResponseWriter, r *http.Request, list api.ManifestList) {
	log.Debug("ShowMultipleChoices", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()))
	msg := ""
	if list.Entries == nil {
		RespondError(w, r, "Could not resolve", http.StatusInternalServerError)
		return
	}
	requestUri := strings.TrimPrefix(r.RequestURI, "/")

	uri, err := api.Parse(requestUri)
	if err != nil {
		RespondError(w, r, "Bad Request", http.StatusBadRequest)
	}

	uri.Scheme = "bzz-list"
	msg += fmt.Sprintf("Disambiguation:<br/>Your request may refer to multiple choices.<br/>Click <a class=\"orange\" href='"+"/"+uri.String()+"'>here</a> if your browser does not redirect you within 5 seconds.<script>setTimeout(\"location.href='%s';\",5000);</script><br/>", "/"+uri.String())
	RespondTemplate(w, r, "error", msg, http.StatusMultipleChoices)
}

func RespondTemplate(w http.ResponseWriter, r *http.Request, templateName, msg string, code int) {
	log.Debug("RespondTemplate", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()))
	respond(w, r, &ResponseParams{
		Code:      code,
		Msg:       template.HTML(msg),
		Timestamp: time.Now().Format(time.RFC1123),
		template:  TemplatesMap[templateName],
	})
}

func RespondError(w http.ResponseWriter, r *http.Request, msg string, code int) {
	log.Debug("RespondError", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()), "code", code)
	RespondTemplate(w, r, "error", msg, code)
}

func respond(w http.ResponseWriter, r *http.Request, params *ResponseParams) {

	w.WriteHeader(params.Code)

	if params.Code >= 400 {
		w.Header().Del("Cache-Control")
		w.Header().Del("ETag")
	}

	acceptHeader := r.Header.Get("Accept")
 /*
 
  
   
  
 
  
 
  
 



 
 
 
 
  
 



 
 
 
 



 
 
 
 
 
 
 
 

