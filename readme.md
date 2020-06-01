Hync is a wrapper and management interface for the core web service daemon at https://github.com/hiroapp-com/diffsync

- Install go and PostgreSQL.

Add the following packages to your go installation:

github.com/gorilla/websocket
github.com/sergi/go-diff/diffmatchpatch (standing on the shoulder of giants...)
github.com/sushimako/rollbar (or your own Rollbar setup and change diffsync's context.go)
github.com/hiroapp-com/diffsync (the core diff match patch sync engine)

Next jump into the diffsync folder above and create the database by running all sql commands in sql/ (eg 'sudo -u postgres find sql/ -name \*.sql -exec psql -f {} \;')

Set the following environment variables:

- SENDWITHUS_KEY
- MANDRILL_KEY
- TWILIO_SID
- TWILIO_TOKEN