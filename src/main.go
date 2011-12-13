package main

import (
	"launchpad.net/gobson/bson"
	"launchpad.net/mgo"
	"artichoke"
	"net/http"
	"net/url"
	"encoding/json"
	"strconv"
	"fmt"
	"strings"
	"encoding/hex"
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

func decodeId(strId string, query url.Values, w http.ResponseWriter) (interface{}, bool) {
	var id interface{}
	// default to hex if hexId isn't present
	if query.Get("hexId") == "false" {
		id = strId
	} else {
		str, err := hex.DecodeString(strId)
		if err != nil {
			writeError(w, 500, "Unable to convert supplied string to hex format")
			return nil, true
		} else if !bson.ObjectId(str).Valid() {
			writeError(w, 400, "Invalid hex string. Maybe you meant to pass the query param hexId=false?")
			return nil, true
		}

		id = bson.ObjectId(str)
	}

	return id, false
}

func insert(data interface{}, c *mgo.Collection, w http.ResponseWriter) {
	err := c.Insert(data)
	if err != nil {
		writeError(w, 500, "Document insertion failed")
		return
	}

	res, err := json.Marshal(data)
	if err != nil {
		writeError(w, 500, "Error stringifying query result")
		return
	}

	writeJSON(w, 201, string(res))
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

				query := m["query"].(url.Values)
				params := m["params"].(map[string]string)

				c := session.DB(params["db"]).C(params["collection"])

				id, resEnded := decodeId(params["docid"], query, w)
				if (resEnded) {
					return true
				}

				// we'll need this query later
				q := c.Find(bson.M{"_id": id})
				n, err := q.Count()
				if err != nil {
					writeError(w, 500, "Error counting results")
					return true
				} else if n == 0 {
					writeError(w, 404, "Document not found")
					return true
				}

				// we don't know the structure before-hand, but we know it's a JSON object
				var out map[string]interface{}
				err = q.One(&out)
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

				params := m["params"].(map[string]string)
				query := m["query"].(url.Values)

				// get the collection the user specified
				c := session.DB(params["db"]).C(params["collection"])

				// check for query params
				if len(query) > 0 {
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

		/* DELETE Requests */

		artichoke.Route{
			Method: "DELETE",
			Pattern: "/:db/:collection/:docid",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				if m["bodyJson"] != nil {
					writeError(w, 400, "DELETE requests on document ids takes no parameters, and thus should have no body or query parameters.")
					return true
				}

				// create a new session per connection
				s := session.New()
				defer s.Close()

				// get the params and query objects
				query := m["query"].(url.Values)
				params := m["params"].(map[string]string)

				c := s.DB(params["db"]).C(params["collection"])

				id, resEnded := decodeId(params["docid"], query, w)
				if (resEnded) {
					return true
				}

				obj := bson.M{"_id": id}

				n, err := c.Find(obj).Count()
				if err != nil {
					writeError(w, 500, "Error communicating with database")
					return true
				}

				if n == 0 {
					writeError(w, 404, "The id provided was not found in this database. Either the document has been deleted already or it never existed.")
					return true
				}

				err = c.Remove(obj)
				if err != nil {
					writeError(w, 500, "Error removing item from database")
					return true
				}

				w.Header().Add("Content-Length", "0")
				w.WriteHeader(204)
				return true
			},
		},
		artichoke.Route{
			Method: "DELETE",
			Pattern: "/:db/:collection",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				if m["bodyJson"] != nil {
					writeError(w, 500, "DELETE requests with parameters is not supported yet")
					return true
				}

				// create a new session per connection
				s := session.New()
				defer s.Close()

				params := m["params"].(map[string]string)
				err := s.DB(params["db"]).C(params["collection"]).DropCollection()
				if err != nil {
					writeError(w, 500, "Error dropping collection")
					return true
				}

				w.Header().Add("Content-Length", "0")
				w.WriteHeader(204)
				return true
			},
		},
		artichoke.Route{
			Method: "DELETE",
			Pattern: "/:db",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				if m["bodyJson"] != nil {
					writeError(w, 500, "DELETE requests with parameters is not supported yet")
					return true
				}

				// create a new session per connection
				s := session.New()
				defer s.Close()

				params := m["params"].(map[string]string)
				err := s.DB(params["db"]).DropDatabase()
				if err != nil {
					writeError(w, 500, "Error dropping database")
					return true
				}

				w.Header().Add("Content-Length", "0")
				w.WriteHeader(204)
				return true
			},
		},

		/* POST Requests */

		// Update a document in place. This will return an error if the document does not exist.
		artichoke.Route{
			Method: "POST",
			Pattern: "/:db/:collection",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				fmt.Println("Post handler called")

				// create a new session per connection
				s := session.New()
				defer s.Close()

				// get the params and query objects
				params := m["params"].(map[string]string)
				query := m["query"].(url.Values)

				// get the colletion
				c := s.DB(params["db"]).C(params["collection"])

				body := m["bodyJson"].(map[string]interface{})
				if body["_id"] != nil {
					id, resEnded := decodeId(body["_id"].(string), query, w)
					if (resEnded) {
						return true
					}

					// ensure that body has the correct id
					body["_id"] = id

					// update, if possible
					err := c.Update(bson.M{"_id": id}, body)
					if err == mgo.NotFound {
						insert(body, &c, w)
						return true
					} else if err != nil {
						writeError(w, 500, "Error updating document in database")
						return true
					} else {
						// update succeeded
						// TODO: make this return the current data in the database?
						w.Header().Add("Content-Length", "0")
						w.WriteHeader(204)
						return true
					}
				} else {
					// generate a new ObjectId
					body["_id"] = bson.NewObjectId()

					// and create it
					insert(body, &c, w)
					return true
				}

				return false
			},
		},

		/* PUT Requests */

		// Since databases and collections are created on-the-fly, there's no PUT for a database or collection.
		//
		// A PUT request will update the URI (overwriting an existing resource), so Insert is used.
		// Note that the document id is mandatory. For an auto-generated ID, POST to the collection instead.
		//
		// For updates, use POST instead (on an existing resource).
		artichoke.Route{
			Method: "PUT",
			Pattern: "/:db/:collection/:docid",
			Handler: func(w http.ResponseWriter, r *http.Request, m artichoke.Data) bool {
				_, bodyExists := m["bodyJson"]
				if !bodyExists {
					writeError(w, 400, "No valid body supplied or body sent with the wrong content-type. Must send as application/json.")
					return true
				}

				// create a new session per connection
				s := session.New()
				defer s.Close()

				// get the params and query objects
				params := m["params"].(map[string]string)
				query := m["query"].(url.Values)

				id, resEnded := decodeId(params["docid"], query, w)
				if (resEnded) {
					return true
				}

				// get the collection so we can check if the id exists
				c := s.DB(params["db"]).C(params["collection"])

				// try finding as hex
				n, err := c.Find(bson.M{"_id": id}).Count()
				if err != nil {
					writeError(w, 500, "Error communicating with database")
					return true
				}

				// put the id in
				body := m["bodyJson"].(map[string]interface{})
				body["_id"] = id

				// get the preferred status code
				var status int
				if n > 0 {
					status = 204
				} else {
					status = 201
				}

				// insert into the database
				// we won't use the one defined above, because we don't want to return anything to the user
				err = c.Insert(body)
				if err != nil {
					writeError(w, 500, "Error inserting document into database")
					return true
				}

				// PUT requests should not return anything on success
				w.Header().Add("Content-Length", "0")
				w.WriteHeader(status)
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

	server := artichoke.New(nil, artichoke.QueryParser(), artichoke.BodyParser(1024), artichoke.Router(genRoutes()))
	server.Run(3345, "localhost")

	<-kill
	session.Close()
}
