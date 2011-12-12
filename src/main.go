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

func printJson(j interface{}) string {
	switch j.(type) {
		case bool:
			return strconv.FormatBool(j.(bool))

		case float64:
			return strconv.FormatFloat(j.(float64), 'g', -1, 64)

		case string:
			return "\"" + j.(string) + "\""

		case []interface{}:
			t := j.([]interface{})
			s := make([]string, len(t))

			for i, val := range t {
				s[i + 1] = printJson(val)
			}

			return fmt.Sprintf("[%s]", strings.Join(s, ","))

		case map[string]interface{}:
			m := j.(map[string]interface{})
			s := make([]string, len(m))

			i := 0
			for k, v := range m {
				s[i] = fmt.Sprintf("\"%s\":%s", k, printJson(v))
				i += 1
			}

			return fmt.Sprintf("{%s}", strings.Join(s, ","))

		case nil:
			return "null"
	}

	return ""
}

func genRoutes() []artichoke.Route {
	return []artichoke.Route{
		artichoke.Route{
			Method: "GET",
			Pattern: "/:db/:collection/?:docid?",
			Handler: func (w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				// create a new session based on the global one; this allows different login details
				s := session.New()
				defer s.Close()

				params := m["params"].(map[string]string)
				c := session.DB(params["db"]).C(params["collection"])

				var err error
				var res []byte
				docid := params["docid"]
				if docid != "" {
					var out map[string]interface{}
					err := c.Find(bson.M{"_id": bson.ObjectIdHex(docid)}).One(&out)
					if err != nil {
						w.WriteHeader(500)
						w.Write([]byte("Error getting document by id"))
						w.Write([]byte(""))
						return true
					}

					res, err = json.Marshal(out)
				} else {
					js := m["bodyJson"]
					if js != nil {
						var out []map[string]interface{}
						err := c.Find(js).All(&out)
						if err != nil {
							w.WriteHeader(500)
							w.Write([]byte("Error with query"))
							w.Write([]byte(""))
							return true
						}

						res, err = json.Marshal(out)
					} else {
						var out []map[string]interface{}
						err := c.Find(nil).All(&out)
						if err != nil {
							w.WriteHeader(500)
							w.Write([]byte("Error getting all documents")) 
							w.Write([]byte(""))
							return true
						}

						res, err = json.Marshal(out)
					}
				}

				if err != nil {
					w.WriteHeader(500)
					w.Write([]byte("Error stringifying query result"))
					w.Write([]byte(""))
					return true
				}

				w.WriteHeader(200)
				w.Write(res)
				w.Write([]byte(""))

				return true
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
					w.WriteHeader(500)
					w.Write([]byte("Error getting collection names"))
					w.Write([]byte(""))
					return true
				}

				w.WriteHeader(200)
				w.Write([]byte(fmt.Sprintf("[%s]", strings.Join(names, ","))))
				w.Write([]byte(""))
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
					w.WriteHeader(500)
					w.Write([]byte("Error getting database names"))
					w.Write([]byte(""))
					return true
				}

				w.WriteHeader(200)
				w.Write([]byte(fmt.Sprintf("[%s]", strings.Join(names, ","))))
				w.Write([]byte(""))
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
