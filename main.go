package main;

import (
    _ "bytes"
    _ "errors"
    "net/smtp"
    "regexp"
    "net/http"
    "gopkg.in/alecthomas/kingpin.v2"
    "strings"
    "log"
    "fmt"
)

var (
    app  = kingpin.New("web-relay", "Mail relay accessible from the web.")
    addr = kingpin.Flag("bindaddr", "Address this server should bind to.").Required().String()
    server = kingpin.Flag("server", "Target relay mail server.").Required().String()
    secret = kingpin.Flag("secret", "Secret required to access server.").Default("").String()
)

var (
    // Mail validation regexp, taken from emailregex.com
    MailRegexp = regexp.MustCompile(`(?:[a-z0-9!#$%&'*+/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_{|}~-]+)*|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21-\x5a\x53-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])+)\])`)
)

// NoAuth implements the smtp.Auth interface and skips any authentication
type NoAuth struct {}
func (n NoAuth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
    return "", []byte(""), nil
}
func (n NoAuth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
    return nil, nil
}

// Since this is intended to be an api, we want proper error codes etc.
// handleWithReturn executes the lambda fn, writes the header according to
// status and writes data as the response. This make early returns less annoying.
func handleWithReturn(w http.ResponseWriter, fn func()(status int, data string)) {
    status, data := fn()
    w.WriteHeader(status)
    fmt.Fprintf(w, data)
}

// handlePlain handles simple e-mails originating from 1 sender and
// intended for 1 or more recipients, optionally verifies secret
func handlePlain(w http.ResponseWriter, r *http.Request) {
    handleWithReturn(w, func()(int, string) {
        /////////
        // POST only!
        if r.Method != "POST" {
            return 404, "404 page not found\n"
        }
        r.ParseForm()

        /////////
        // validate post parameters
        // (delete all parameters so we can check for extra ones)

        // check if a secret is used, if yes, match, then fail or continue
        // we only accept either 0 or 1 secrets
        if len(r.Form["secret"]) >= 1 && r.Form["secret"][0] != *secret {
            return 401, "401 forbidden\n"
        }
        delete(r.Form, "secret")

        // check for exactly 1 sender address and verify with regex
        if len(r.Form["from"]) != 1 || !MailRegexp.Match([]byte(r.Form["from"][0])) {
            return 400, "400 bad request (from)\n"
        }
        from := r.Form["from"][0]
        delete(r.Form, "from")

        // check for at least 1 recipient and check all with regex
        if len(r.Form["to"]) < 1 {
            return 400, "400 bad request (to)\n"
        }
        for _, addr := range(r.Form["to"]) {
            if !MailRegexp.Match([]byte(addr)) {
                return 400, "400 bad request (to)\n"
            }
        }
        to := r.Form["to"]
        delete(r.Form, "to")

        // check for exactly 1 non-empty subject
        if len(r.Form["subject"]) != 1 || strings.TrimSpace(r.Form["subject"][0]) == "" {
            return 400, "400 bad request (subject)\n"
        }
        subject := r.Form["subject"][0]
        delete(r.Form, "subject")

        // check for exactly 1 message with no further restrictions
        if len(r.Form["message"]) != 1 || strings.TrimSpace(r.Form["message"][0]) == "" {
            return 400, "400 bad request (message)\n"
        }
        message := r.Form["message"][0]
        delete(r.Form, "message")

        // check for extra flags and fail if there are any
        if len(r.Form) > 0 {
            return 400, "400 bad request (...)"
        }

        /////////
        // log provided info

        log.Printf("from: '%s'", from)
        log.Printf("to: '%s'", to)
        log.Printf("subject: '%s'", subject)
        log.Printf("message:")
        for _, line := range(strings.Split(message, "\n")) {
            log.Printf(">%s", line)
        }

        /////////
        // sent mail

        err := smtp.SendMail(*server, NoAuth{}, from, to, []byte(message))
        if err != nil {
            return 500, "500 error: " + err.Error() + "\n"
        }

        return 200, "200 ok\n"
    })
}

// handleTemplate handles request for templated mails that are expanded
// on the server side. In addition to all parameters that are required
// for a simple plain e-mail, templated e-mails require 1 or more rows
// containing substitutes for the placeholders in the e-mail or subject
// fields. Rows containing substitutes are of the form:
// "name"="value"[,more...]
func handleTemplated(w http.ResponseWriter, r *http.Request) {
    handleWithReturn(w, func()(int, string) {
        /////////
        // POST only!
        if r.Method != "POST" {
            return 404, "404 page not found\n"
        }
        r.ParseForm()

        /////////
        // validate post parameters
        // (delete all parameters so we can check for extra ones)

        // check if a secret is used, if yes, match, then fail or continue
        // we only accept either 0 or 1 secrets
        if len(r.Form["secret"]) >= 1 && r.Form["secret"][0] != *secret {
            return 401, "401 forbidden\n"
        }
        delete(r.Form, "secret")

        // check for exactly 1 sender address and verify with regex
        if len(r.Form["from"]) != 1 || !MailRegexp.Match([]byte(r.Form["from"][0])) {
            return 400, "400 bad request (from)\n"
        }
        from := r.Form["from"][0]
        delete(r.Form, "from")

        // check for at least 1 recipient and check all with regex
        if len(r.Form["to"]) < 1 {
            return 400, "400 bad request (to)\n"
        }
        for _, addr := range(r.Form["to"]) {
            if !MailRegexp.Match([]byte(addr)) {
                return 400, "400 bad request (to)\n"
            }
        }
        to := r.Form["to"]
        delete(r.Form, "to")

        // check for exactly 1 non-empty subject
        if len(r.Form["subject"]) != 1 || strings.TrimSpace(r.Form["subject"][0]) == "" {
            return 400, "400 bad request (subject)\n"
        }
        subject := r.Form["subject"][0]
        delete(r.Form, "subject")

        // check for exactly 1 message with no further restrictions
        if len(r.Form["message"]) != 1 || strings.TrimSpace(r.Form["message"][0]) == "" {
            return 400, "400 bad request (message)\n"
        }
        message := r.Form["message"][0]
        delete(r.Form, "message")

        // check for 1 or more rows with template values
        if len(r.Form["values"]) < 1 {
            return 400, "400 bad request (values)\n"
        }
        var values = make(map[string][]string)
        for _, row := range(r.Form["values"]) {
            // TODO
            _ = row
        }
        _ = values

        // check for extra flags and fail if there are any
        if len(r.Form) > 0 {
            return 400, "400 bad request (...)"
        }

        /////////
        // log provided info

        log.Printf("from: '%s'", from)
        log.Printf("to: '%s'", to)
        log.Printf("subject: '%s'", subject)
        log.Printf("message:")
        for _, line := range(strings.Split(message, "\n")) {
            log.Printf(">%s", line)
        }

        /////////
        // send mail


        return 200, "200 ok\n"
    })
}

func main() {
    kingpin.Parse()

    if *secret == "" {
        log.Printf("CAUTION: NO SECRET REQUIRED, ABUSE LIKELY")
    }

    http.HandleFunc("/plain", handlePlain);
    http.HandleFunc("/template", handleTemplated);

    log.Printf("starting server...")
    log.Fatal(http.ListenAndServe(*addr, nil))
}
