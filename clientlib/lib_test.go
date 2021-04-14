package clientlib
//
//import (
//	"bytes"
//	"io/ioutil"
//	"net/http"
//	"testing"
//)
//
//type RoundTripFunc func(req *http.Request) *http.Response
//
//func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
//	return f(req), nil
//}
//
////TestClient returns *http.Client with Transport replaced to avoid making real calls
//func TestClient(fn RoundTripFunc) *http.Client {
//	return &http.Client{
//		Transport: RoundTripFunc(fn),
//	}
//}
//
//func TestGetAddressInfo(t *testing.T) {
//	client := TestClient(func(req *http.Request) *http.Response {
//		// Test request parameters
//		equals(t, req.URL.String(), "http://example.com/some/path")
//		return &http.Response{
//			StatusCode: 200,
//			// Send response to be tested
//			Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
//			// Must be set to non-nil value or it panics
//			Header:     make(http.Header),
//		}
//	})
//	api := API{client, "http://example.com"}
//	body, err := api.GetAddressInfo("fakeAddress")
//	ok(t, err)
//	equals(t, []byte("OK"), body)
//}
//
//func TestSendCoin(t *testing.T) {
//
//}
//
//func TestGetTransactions(t *testing.T) {
//
//}