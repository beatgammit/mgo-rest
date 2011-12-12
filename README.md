Intro
=====

`mgo-rest` is a RESTful interface to MongoDB. Other REST-like interfaces exist, but this project tries to be completely REST compliant (no stupid hacks). This means that the normal HTTP verbs are used (explained below).

Verbs
=====

`mgo-rest` only uses the standard HTTP verbs: DELETE, GET, HEAD, POST, PUT.

The REST API is modeled after CouchDB, with some adaptations for MongoDB.

DELETE
------

> DELETE /db

This should only be used during testing. If any query parameters are passed, or if there's a body, then the request will be aborted with a 400 error.

This functionality may be removed, or at least disabled for production.

> DELETE /db/collection

This should only be used during testing. If any query parameters are passed (or if there's a body), then the request will be handled as if removing a document.

> DELETE /db/collection/docid

Removes a document from the collection. If docid is not specified, then a search will be done based on the query parameters or body.

Removing a document by ID is preferred, as it is more efficient.

GET
---

> GET /

Gets an array of all available databases. Query parameters are ignored.

> GET /db

Gets an array of all collections in the database specified.

> GET /db/collection

Gets an array of all of the documents in the collection specified.

> GET /db/collection/docid

Gets a document from the collection. If docid is not provided, a search will be done based on the query parameters or body.

HEAD
----

> HEAD /db/collection/docid

Gets information about the document (not the data itself).

POST
----

> POST /db/collection/docid

If the document does not exist, or if no docid is provided, then a new document will be created. If the document does exist, then it is updated in place. Fields are only added or replaced, not removed.

This operation should only be used with a valid docid. If a document is to be created, use PUT instead.

PUT
---

> PUT /db

Creates a database with the provided name.

> PUT /db/collection

Creates a collection in the specified database with the provided name.

> PUT /db/collection/docid

Inserts a new document into the database. If the document already exists, it will be replaced (not updated). If not docid is provided, one will be automatically generated.
