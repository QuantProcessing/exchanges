package sdk

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetKlinesBuildsExpectedQuery(t *testing.T) {
	t.Parallel()

	var gotQuery map[string]string
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotQuery = map[string]string{
				"symbol":    r.URL.Query().Get("symbol"),
				"interval":  r.URL.Query().Get("interval"),
				"startTime": r.URL.Query().Get("startTime"),
				"endTime":   r.URL.Query().Get("endTime"),
				"priceType": r.URL.Query().Get("priceType"),
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := client.GetKlines(context.Background(), "BTC_USDC", "1month", 1710000000, 1710003600, "Last")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"symbol":    "BTC_USDC",
		"interval":  "1month",
		"startTime": "1710000000",
		"endTime":   "1710003600",
		"priceType": "Last",
	}, gotQuery)
}

func TestClientGetDepthDelegatesToGetOrderBook(t *testing.T) {
	t.Parallel()

	assertSingleMethodWrapperCall(t, "public_rest.go", "GetDepth", "GetOrderBook", "ctx", "symbol", "limit")
}

func TestClientGetOrderBookDelegatesToDepthEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotQuery map[string]string
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotQuery = map[string]string{
				"symbol": r.URL.Query().Get("symbol"),
				"limit":  r.URL.Query().Get("limit"),
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"asks":[],"bids":[],"lastUpdateId":"7","timestamp":123}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	book, err := client.GetOrderBook(context.Background(), "BTC_USDC", 20)
	require.NoError(t, err)
	require.Equal(t, "/api/v1/depth", gotPath)
	require.Equal(t, map[string]string{
		"symbol": "BTC_USDC",
		"limit":  "20",
	}, gotQuery)
	require.Equal(t, "7", book.LastUpdateID)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func assertSingleMethodWrapperCall(t *testing.T, filename, wrapperName, targetName string, argNames ...string) {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filepath.Join(filepath.Dir(currentFile), filename), nil, 0)
	require.NoError(t, err)

	var method *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != wrapperName {
			continue
		}
		method = fn
		break
	}
	require.NotNil(t, method, "wrapper %s not found", wrapperName)
	require.Len(t, method.Body.List, 1, "wrapper %s should stay a single statement", wrapperName)

	ret, ok := method.Body.List[0].(*ast.ReturnStmt)
	require.True(t, ok, "wrapper %s should consist of a return statement", wrapperName)
	require.Len(t, ret.Results, 1, "wrapper %s should return a single result expression", wrapperName)

	call, ok := ret.Results[0].(*ast.CallExpr)
	require.True(t, ok, "wrapper %s should return a method call", wrapperName)

	selector, ok := call.Fun.(*ast.SelectorExpr)
	require.True(t, ok, "wrapper %s should call a selector", wrapperName)
	receiver, ok := selector.X.(*ast.Ident)
	require.True(t, ok, "wrapper %s should call through the receiver", wrapperName)
	require.Equal(t, "c", receiver.Name)
	require.Equal(t, targetName, selector.Sel.Name)
	require.Len(t, call.Args, len(argNames), "wrapper %s should preserve its argument list", wrapperName)
	for i, argName := range argNames {
		arg, ok := call.Args[i].(*ast.Ident)
		require.True(t, ok, "wrapper %s arg %d should stay an identifier", wrapperName, i)
		require.Equal(t, argName, arg.Name)
	}
}
