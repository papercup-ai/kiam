// Copyright 2017 uSwitch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package metadata

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
)

type proxyHandler struct {
	backingService   http.Handler
	allowRouteRegexp *regexp.Regexp
}

var tokenRouteRegexp = regexp.MustCompile("^/?[^/]+/api/token$")

func (p *proxyHandler) Install(router *mux.Router) {
	router.PathPrefix("/").Handler(adapt(withMeter("proxy", p)))
}

type teeWriter struct {
	http.ResponseWriter
	status int
}

func (w *teeWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (p *proxyHandler) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request) (int, error) {
	if p.allowRouteRegexp.MatchString(r.URL.Path) {
		writer := &teeWriter{w, http.StatusOK}
		r.RemoteAddr = ""
		p.backingService.ServeHTTP(writer, r)

		if writer.status == http.StatusOK {
			success.WithLabelValues("proxy").Inc()
		}
		return writer.status, nil
	}

	proxyDenies.Inc()
	return http.StatusNotFound, fmt.Errorf("request blocked by allow-route-regexp %q: %s", p.allowRouteRegexp, r.URL.Path)
}

func newProxyHandler(backingService http.Handler, allowRouteRegexp *regexp.Regexp) *proxyHandler {
	if allowRouteRegexp.String() == "" {
		allowRouteRegexp = regexp.MustCompile("^$")
	}
	return &proxyHandler{
		backingService:   backingService,
		allowRouteRegexp: allowRouteRegexp,
	}
}
