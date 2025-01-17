// Copyright Splunk Inc.
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

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/signalfx/splunk-otel-go/distro"
	"github.com/signalfx/splunk-otel-go/instrumentation/net/http/splunkhttp"
)

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	// handle CTRL+C gracefully
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// initialize Splunk OTel distro
	sdk, err := distro.Run()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := sdk.Shutdown(context.Background()); err != nil {
			log.Println(err)
			exitCode = 1
		}
	}()

	r := mux.NewRouter()

	// instrument http.Handler
	r.Use(otelmux.Middleware("mux-server"))
	r.Use(func(h http.Handler) http.Handler { return splunkhttp.NewHandler(h) })

	r.HandleFunc("/hello", func(rw http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(rw, "Hello there.") // ignore the error
	}).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErrCh <- err
		} else {
			srvErrCh <- nil
		}
	}()

	<-ctx.Done()
	stop() // stop receiving signal notifications; next interrupt signal should kill the application

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Println(err)
		exitCode = 1
		return
	}
	if err := <-srvErrCh; err != nil {
		log.Println(err)
		exitCode = 1
		return
	}
}
