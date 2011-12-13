package main

import (
	"launchpad.net/gobson/bson"
	"launchpad.net/mgo"
	"artichoke"
	"net/http"
	"encoding/json"
	"strconv"
	"fmt"
	"strings"
)

var session *mgo.Session
var kill chan bool = make(chan bool)

func writeJSON(w http.ResponseWriter, status int, message string) {
	// write headers
	header := w.Header()
	header.Add("Content-Length", strconv.Itoa(len(message)))
	header.Add("Content-Type", "application/json")

	// write status code
	w.WriteHeader(status)

	// write data
	w.Write([]byte(message))
}

func writeError(w http.ResponseWriter, status int, message string) {
	// do the same as a regular response, but manually format as JSON
	writeJSON(w, status, fmt.Sprintf("{\"error\":\"%s\"}", message))
}

func genRoutes() []artichoke.Route {

	/*
		Order of the requests are important. They will be executed top-down.
	 */

	return []artichoke.Route{

		/* GET requests */

		artichoke.Route{
			Method: "GET",
			Pattern: "/:db/:collection/:docid",
			Handler: func (w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				// create a new session based on the global one; this allows different login details
				s := session.New()
				defer s.Close()

				params := m["params"].(map[string]string)

				// we don't know the structure before-hand, but we know it's a JSON object
				var out map[string]interface{}
				err := session.DB(params["db"]).C(params["collection"]).Find(bson.M{"_id": bson.ObjectIdHex(params["docid"])}).One(&out)
				if err != nil {
					writeError(w, 500, "Error getting document by id")
					return true
				}

				res, err := json.Marshal(out)
				if err != nil {
					writeError(w, 500, "Error stringifying query result")
					return true
				}

				writeJSON(w, 200, string(res))
				return true
			},
		},
		artichoke.Route{
			Method: "GET",
			Pattern: "/:db/:collection",
			Handler: func (w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				// create a new session based on the global one; this allows different login details
				s := session.New()
				defer s.Close()

				// get the collection the user specified
				params := m["params"].(map[string]string)
				c := session.DB(params["db"]).C(params["collection"])

				// check for query params
				js := m["query"]
				if js != nil {
					// not yet implemented
					writeError(w, 500, "Query params not supported yet")
					return true
				} else {
					// no query parameters means they want everything in the collection
					var out []map[string]interface{}
					err := c.Find(nil).All(&out)
					if err != nil {
						writeError(w, 500, "Error getting all documents")
						return true
					}

					res, err := json.Marshal(out)
					if err != nil {
						writeError(w, 500, "Error stringifying response")
						return true
					}

					writeJSON(w, 200, string(res))
					return true
				}

				return false
			},
		},
		artichoke.Route{
			Method: "GET",
			Pattern: "/:db",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				// create a new session per connection
				s := session.New()
				defer s.Close()

				params := m["params"].(map[string]string)
				names, err := session.DB(params["db"]).CollectionNames()
				if err != nil {
					writeError(w, 500, "Error getting collection names")
					return true
				}

				writeJSON(w, 200, fmt.Sprintf("[%s]", strings.Join(names, ",")))
				return true
			},
		},
		artichoke.Route{
			Method: "GET",
			Pattern: "/",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				// create a new session per connection
				s := session.New()
				defer s.Close()

				names, err := s.DatabaseNames()
				if err != nil {
					writeError(w, 500, "Error getting database names")
					return true
				}

				writeJSON(w, 200, fmt.Sprintf("[%s]", strings.Join(names, ",")))
				return true
			},
		},
	};
}

func main() {
	var err error
	session, err = mgo.Mongo("localhost:27017")
	if err != nil {
		panic(err)
	}

	server := artichoke.New(nil, artichoke.BodyParser(1024), artichoke.Router(genRoutes()))
	server.Run(3345, "localhost")

	<-kill
	session.Close()
}
