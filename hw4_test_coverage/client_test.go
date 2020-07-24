package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const token = "test token"

var content Content
var testServer = httptest.NewServer(http.HandlerFunc(SearchServer))

type Content struct {
	Users []Row `xml:"row"`
}

type Row struct {
	Id        int `xml:"id"`
	Name      string
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type TestCase struct {
	Request SearchRequest
	Result  Result
}

type Result struct {
	Response *SearchResponse
	Err      error
}

type ByID []Row

type ByAge []Row

type ByName []Row

func (s ByID) Len() int {
	return len(s)
}

func (s ByID) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByID) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

func (s ByAge) Len() int {
	return len(s)
}

func (s ByAge) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByAge) Less(i, j int) bool {
	return s[i].Age < s[j].Age
}

func (s ByName) Len() int {
	return len(s)
}

func (s ByName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func init() {
	file, err := os.Open("dataset.xml")
	if err != nil {
		panic(err)
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	err = xml.Unmarshal(fileContents, &content)
	if err != nil {
		panic(err)
	}

	for i, _ := range content.Users {
		content.Users[i].Name = content.Users[i].FirstName + " " + content.Users[i].LastName
	}
}

func SearchServer(writer http.ResponseWriter, r *http.Request) {
	// limit, _ :=  strconv.Atoi(r.FormValue("limit"))
	// offset, _ := strconv.Atoi(r.FormValue("offset"))
	query := r.FormValue("query")
	orderField := r.FormValue("order_field")
	orderBy, err := strconv.Atoi(r.FormValue("order_by"))
	if err != nil {
		fmt.Errorf("Error converting 'order by'")
	}

	if r.URL.Path == "/timeout" {
		writer.WriteHeader(http.StatusFound)
		time.Sleep(2 * time.Second)
	}

	if r.URL.Path == "/cant-unpack-json" {
		writer.WriteHeader(http.StatusNotFound)
		writer.Write([]byte("{]"))
		return
	}

	if r.URL.Path == "/internal-server-error" {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if r.URL.Path == "/bad-request-json-unpack-error" {
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte("{]"))
		return
	}

	var findRows []Row
	for _, row := range content.Users {
		if strings.Contains(row.Name, query) || strings.Contains(row.About, query) || len(query) == 0 {
			findRows = append(findRows, row)
		}
	}

	if orderBy != 0 {
		switch orderField {
		case "ID":
			sort.Sort(ByID(findRows))
		case "Age":
			sort.Sort(ByAge(findRows))
		case "Name", "":
			sort.Sort(ByName(findRows))
		default:
			json, _ := json.Marshal(SearchErrorResponse{"ErrorBadOrderField"})
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write(json)
			return
		}

		if orderBy == -1 {
			for i, j := 0, len(findRows)-1; i < j; i, j = i+1, j-1 {
				findRows[i], findRows[j] = findRows[j], findRows[i]
			}
		} else {
			json, _ := json.Marshal(SearchErrorResponse{"Unexpected 'order by'"})
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write(json)
			return
		}
	}
	accessToken := r.Header.Get("AccessToken")
	if accessToken != token {
		json, err := json.Marshal(SearchErrorResponse{"Bad AccessToken"})
		if err != nil {
			fmt.Errorf("can't marshal error")
		}
		writer.WriteHeader(http.StatusUnauthorized)
		writer.Write(json)
		return
	}

	json, err := json.Marshal(findRows)
	if err != nil {
		fmt.Errorf("cant unpack result json")
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write(json)
}

func TestFindUsersNextPage(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      1,
			Offset:     0,
			Query:      "May",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			&SearchResponse{
				Users: []User{
					User{
						Id:     6,
						Name:   "Jennings Mays",
						Age:    39,
						About:  "Veniam consectetur non non aliquip exercitation quis qui. Aliquip duis ut ad commodo consequat ipsum cupidatat id anim voluptate deserunt enim laboris. Sunt nostrud voluptate do est tempor esse anim pariatur. Ea do amet Lorem in mollit ipsum irure Lorem exercitation. Exercitation deserunt adipisicing nulla aute ex amet sint tempor incididunt magna. Quis et consectetur dolor nulla reprehenderit culpa laboris voluptate ut mollit. Qui ipsum nisi ullamco sit exercitation nisi magna fugiat anim consectetur officia.\n",
						Gender: "male",
					},
				},
				NextPage: true,
			},
			nil,
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL,
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if err != nil || !reflect.DeepEqual(testCase.Result.Response, result) {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersLimitLess(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      -5,
			Offset:     0,
			Query:      "Jennings",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("limit must be > 0"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL,
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersOffset(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      5,
			Offset:     -1,
			Query:      "Jennings",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("offset must be > 0"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL,
	}
	result, err := searchClient.FindUsers(testCase.Request)

	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersLimitMore(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "234",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: &SearchResponse{
				Users:    []User{},
				NextPage: false,
			},
			Err: nil,
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL,
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result == nil || err != nil || result.NextPage != false {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersBadRequest(t *testing.T) {
	testCases := []TestCase{
		TestCase{
			Request: SearchRequest{
				Limit:      5,
				Offset:     5,
				Query:      "Jennings",
				OrderField: "Bad",
				OrderBy:    OrderByAsc,
			},
			Result: Result{
				Response: nil,
				Err:      errors.New("OrderFeld Bad invalid"),
			},
		},
		TestCase{
			Request: SearchRequest{
				Limit:      5,
				Offset:     9,
				Query:      "Jennings",
				OrderField: "",
				OrderBy:    123,
			},
			Result: Result{
				Response: nil,
				Err:      errors.New("unknown bad request error: Unexpected 'order by'"),
			},
		},
	}

	for _, testCase := range testCases {
		searchClient := &SearchClient{
			AccessToken: token,
			URL:         testServer.URL,
		}
		result, err := searchClient.FindUsers(testCase.Request)
		if result != nil || err.Error() != testCase.Result.Err.Error() {
			printError(testCase, result, err, t)
		}
	}
}

func TestFindUsersUnauthorized(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("Bad AccessToken"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: "",
		URL:         testServer.URL,
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersTimeout(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("timeout for limit=26&offset=1&order_by=-1&order_field=&query="),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL + "/timeout",
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersUnknownProtocol(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("unknown error Get \"protounknown://test?limit=26&offset=1&order_by=-1&order_field=&query=\": unsupported protocol scheme \"protounknown\""),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         "protounknown://test",
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersCantUnpackJson(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("cant unpack result json: invalid character ']' looking for beginning of object key string"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL + "/cant-unpack-json",
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersInternalServerError(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("SearchServer fatal error"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL + "/internal-server-error",
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func TestFindUsersBadRequestUnpackError(t *testing.T) {
	testCase := TestCase{
		Request: SearchRequest{
			Limit:      50,
			Offset:     1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsc,
		},
		Result: Result{
			Response: nil,
			Err:      errors.New("cant unpack error json: invalid character ']' looking for beginning of object key string"),
		},
	}

	searchClient := &SearchClient{
		AccessToken: token,
		URL:         testServer.URL + "/bad-request-json-unpack-error",
	}
	result, err := searchClient.FindUsers(testCase.Request)
	if result != nil || err.Error() != testCase.Result.Err.Error() {
		printError(testCase, result, err, t)
	}
}

func printError(testCase TestCase, result *SearchResponse, err error, t *testing.T) {
	t.Errorf("wrong result, \n\nexpected result: %#v\n error: %#v,\n\n got result: %#v, \n error: %#v", testCase.Result.Response, testCase.Result.Err, result, err)
}
