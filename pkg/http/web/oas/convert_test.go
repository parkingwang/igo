package oas

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestParseDeep(t *testing.T) {

	type Emb struct {
		Name string `json:"name" comment:"nihao" binding:"required"`
		Sex  bool   `json:"sex" comment:"sex.."`
	}

	type s struct {
		Emb
		ID int64       `json:"id" comment:"id..."`
		V  interface{} `json:"v" binding:"required" comment:"value"`
	}

	x := s{}
	x.Emb.Name = "afocus"
	x.ID = 123

	v := map[string]Schema{}
	p := parseDeep(reflect.ValueOf(x), "schema", "json", v)

	for k, v := range p["schema"].Properties {
		fmt.Println(k, "--->", v.Properties)
	}

	k, _ := json.MarshalIndent(p, "", " ")
	fmt.Println(string(k))

}
