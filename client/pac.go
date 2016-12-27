package client

import "net/http"

func (cmd *Command) servPac(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`function FindProxyForURL(url, host)
{
	return "PROXY 127.0.0.1:8780";
}`))
}
