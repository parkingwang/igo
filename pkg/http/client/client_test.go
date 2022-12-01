package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

var testClient = MustClient(Option{
	ParseResponse: func(r *http.Response, a any) error {
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		if r.StatusCode < 400 {
			return dec.Decode(a)
		} else {
			fmt.Println(r.StatusCode)
			var v struct{ Message string }
			if err := dec.Decode(&v); err != nil {
				fmt.Println(err)
				return err
			}
			return errors.New(v.Message)
		}
	},
	BaseURL: "https://api.github.com",
})

func TestRequest(t *testing.T) {
	var repo struct {
		ID       int64  `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	}

	if err := testClient.Get("/repos/golang/go").
		Do(context.Background(), &repo); err == nil {
		if repo.FullName != "golang/go" {
			t.FailNow()
		}
	}
}

func TestRequestResponseErr(t *testing.T) {
	if err := testClient.Get("/repos/golang/go1111").Do(context.Background(), nil); err != nil {
		if err.Error() != "Not Found" {
			t.FailNow()
		}
	} else {
		fmt.Println("22")
		t.FailNow()
	}
}
