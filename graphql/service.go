// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/node"
)

const (
	// maxQueryDepth limits the maximum field nesting depth allowed in GraphQL queries.
	// Upstream 1c74f2376 (#32344).
	maxQueryDepth = 20

	// maxRequestBodySize limits the size of incoming GraphQL request bodies, matching the
	// JSON-RPC body limit. Upstream c782197d4 (#35034).
	maxRequestBodySize = 5 * 1024 * 1024

	// queryTimeout bounds the execution of a single GraphQL query.
	// Upstream ee9ff0646 (#26116).
	queryTimeout = 60 * time.Second
)

type handler struct {
	Schema *graphql.Schema
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}
	// Decode stops after the first JSON value, so EOF is required afterwards to make sure
	// oversized trailing data isn't silently ignored.
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&params); err != nil {
		writeRequestError(w, err)
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected content after JSON value")
		}
		writeRequestError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()

	response := h.Schema.Exec(ctx, params.Query, params.OperationName, params.Variables)
	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(response.Errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)

}

// writeRequestError reports a body-decoding failure, distinguishing an over-sized body
// from ordinary malformed JSON. Upstream c782197d4 (#35034).
func writeRequestError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// New constructs a new GraphQL service instance.
func New(stack *node.Node, backend ethapi.Backend, cors, vhosts []string) error {
	if backend == nil {
		panic("missing backend")
	}
	// check if http server with given endpoint exists and enable graphQL on it
	_, err := newHandler(stack, backend, cors, vhosts)
	return err
}

// newHandler returns a new `http.Handler` that will answer GraphQL queries.
// It additionally exports an interactive query browser on the / endpoint.
func newHandler(stack *node.Node, backend ethapi.Backend, cors, vhosts []string) (*handler, error) {
	q := Resolver{backend}

	// Upstream 1c74f2376: cap the nesting depth so a deeply nested query cannot burn CPU
	// and memory before the execution timeout kicks in.
	s, err := graphql.ParseSchema(schema, &q, graphql.MaxDepth(maxQueryDepth))
	if err != nil {
		return nil, err
	}
	h := handler{Schema: s}
	handler := node.NewHTTPHandlerStack(h, cors, vhosts)

	stack.RegisterHandler("GraphQL UI", "/graphql/ui", GraphiQL{})
	stack.RegisterHandler("GraphQL", "/graphql", handler)
	stack.RegisterHandler("GraphQL", "/graphql/", handler)

	return &h, nil
}
