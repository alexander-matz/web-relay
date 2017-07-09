package main

import (
	_ "bytes"
	"encoding/csv"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
)

var (
	app    = kingpin.New("web-relay", "Mail relay accessible from the web.")
	addr   = kingpin.Flag("bindaddr", "Address this server should bind to.").Required().String()
	server = kingpin.Flag("server", "Target relay mail server.").Required().String()
	secret = kingpin.Flag("secret", "Secret required to access server.").Default("").String()
)

var (
	// Mail validation regexp, taken from emailregex.com
	MailRegexp = regexp.MustCompile(`(?:[a-z0-9!#$%&'*+/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_{|}~-]+)*|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21-\x5a\x53-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])+)\])`)
	VarRegexp  = regexp.MustCompile(`\${\s*([^}\n \t\r]*)\s*}`)
)

// NoAuth implements the smtp.Auth interface and skips any authentication
type NoAuth struct{}

func (n NoAuth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
	return "", []byte(""), nil
}
func (n NoAuth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
	return nil, nil
}

// Apply template to a set of variables, fail if variable does not exist
func DoTemplate(templ string, values map[string]string) (string, error) {
	var err error = nil
	res := VarRegexp.ReplaceAllStringFunc(templ, func(txt string) string {
		// golang regex replace kind of sucks (no info about submatches), so we have
		// to match twice
		var name = VarRegexp.FindStringSubmatch(txt)[1]
		if val, ok := values[name]; ok {
			return val
		} else {
			err = fmt.Errorf("variable '%s' not found", name)
			return ""
		}
	})
	return res, err
}

type Mail struct {
    From string
    To []string
    Subject string
    Message string
}
func NewMail(from string, to []string, subject string, message string) (*Mail, error) {
    if !MailRegexp.MatchString(from) {
        return nil, fmt.Errorf("sender address '%s' invalid", from)
    }
    if len(to) < 1 {
        return nil, fmt.Errorf("no receiver provided")
    }
    for _, addr := range(to) {
        if !MailRegexp.MatchString(addr) {
            return nil, fmt.Errorf("receiver address '%s' invalid", addr)
        }
    }
    if strings.TrimSpace(subject) == "" {
        return nil, fmt.Errorf("subject is empty")
    }
    if strings.TrimSpace(message) == "" {
        return nil, fmt.Errorf("message is empty")
    }
    return &Mail{
        From: from,
        To: to,
        Subject: subject,
        Message: message,
    }, nil
}

func MailFromStringMap(vals map[string][]string) (*Mail, error) {
    if len(vals["from"]) != 1 {
        return nil, fmt.Errorf("from")
    }
    if len(vals["to"]) < 1 {
        return nil, fmt.Errorf("to")
    }
    if len(vals["subject"]) != 1 {
        return nil, fmt.Errorf("subject")
    }
    if len(vals["message"]) != 1 {
        return nil, fmt.Errorf("message")
    }
    from := vals["from"][0]
    to := vals["to"]
    subject := vals["subject"][0]
    message := vals["message"][0]

    return NewMail(from, to, subject, message)
}

// Since this is intended to be an api, we want proper error codes etc.
// handleWithReturn executes the lambda fn, writes the header according to
// status and writes data as the response. This make early returns less annoying.
func handleWithReturn(w http.ResponseWriter, fn func() (status int, data string)) {
	status, data := fn()
	w.WriteHeader(status)
	fmt.Fprintf(w, data)
}

// handlePlain handles simple e-mails originating from 1 sender and
// intended for 1 or more recipients, verifies secret if required.
//
// POST parameters:
// secret: 0 or 1 strings
// from: 1 string, must pass mail regex
// to: 1 or more strings, all must pass mail regex
// subject: 1 string, no requirements
// message: 1 string, no requirements
func handlePlain(w http.ResponseWriter, r *http.Request) {
	handleWithReturn(w, func() (int, string) {
		/////////
		// POST only!
		if r.Method != "POST" {
			return 405, "405 method not allowed\n"
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

        mail, err := MailFromStringMap(r.Form)
        if err != nil {
            return 400, "400 bad request (" + err.Error() + ")"
        }

        // delete all possible keys and check if there are any left
        delete(r.Form, "secret")
        delete(r.Form, "from")
        delete(r.Form, "to")
        delete(r.Form, "subject")
        delete(r.Form, "message")

		// check for extra flags and fail if there are any
		if len(r.Form) > 0 {
			return 400, "400 bad request (extra arguments)"
		}

		/////////
		// log provided info

		log.Printf("from: '%s'", mail.From)
		log.Printf("to: '%s'", mail.To)
		log.Printf("subject: '%s'", mail.Subject)
		log.Printf("message:")
		for _, line := range strings.Split(mail.Message, "\n") {
			log.Printf(">%s", line)
		}

		/////////
		// sent mail

        /*
		err := smtp.SendMail(*server, NoAuth{}, from, to, []byte(message))
		if err != nil {
			return 500, "500 error: " + err.Error() + "\n"
		}
        */

		return 200, "200 ok\n"
	})
}

type TemplateRequest struct {
    currIndex int
    From string
    ToTempl []string
    SubjectTempl string
    MessageTempl string
    Headers []string
    Records [][]string
}
// Validates number of fields and builds TemplateRequest
func TemplateFromStringMap(vals map[string][]string) (*TemplateRequest, error) {
    if len(vals["from"]) != 1 {
        return nil, fmt.Errorf("from")
    }
    if len(vals["toTemplate"]) < 1 {
        return nil, fmt.Errorf("toTemplate")
    }
    if len(vals["subjectTemplate"]) != 1 {
        return nil, fmt.Errorf("subjectTemplate")
    }
    if len(vals["messageTemplate"]) != 1 {
        return nil, fmt.Errorf("messageTemplate")
    }
    if len(vals["records"]) < 1 {
        return nil, fmt.Errorf("records")
    }

    from := vals["from"][0]
    toTemplate := vals["toTemplate"]
    subjectTemplate := vals["subjectTemplate"][0]
    messageTemplate := vals["messageTemplate"][0]

    var headers []string
    var records [][]string

    {
        var stringIn = strings.Join(vals["records"], "\n")
        var stringReader = strings.NewReader(stringIn)
        var csvReader = csv.NewReader(stringReader)
        records, err := csvReader.ReadAll()
        if len(records) < 1 {
            return nil, fmt.Errorf("no data")
        }
        if err != nil {
            return nil, err
        }
        headers = records[0]
        records = records[1:]
    }

    var keyCheck = make(map[string]bool)
    for _, key := range(headers) {
        if _, exists := keyCheck[key]; exists {
            return nil, fmt.Errorf("duplicate fields")
        }
        keyCheck[key] = true
    }

    return &TemplateRequest{
        currIndex: -1,
        From: from,
        ToTempl: toTemplate,
        SubjectTempl: subjectTemplate,
        MessageTempl: messageTemplate,
        Headers: headers,
        Records: records,
    }, nil
}

func (r *TemplateRequest) Next() bool {
    r.currIndex += 1
    return r.currIndex < len(r.Records)
}

func (r *TemplateRequest) GetMail() (*Mail, error) {
    idx := r.currIndex
    if idx < 0 || idx >= len(r.Records) {
        return nil, fmt.Errorf("TemplateRequest invalid state")
    }

    values := make(map[string]string)
    for fieldIdx, name := range(r.Headers) {
        values[name] = r.Records[idx][fieldIdx]
    }

    from := r.From

    to := make([]string, len(r.ToTempl))
    for toIdx, addrTempl := range(r.ToTempl) {
        addr, err := DoTemplate(addrTempl, values)
        if err != nil {
            return nil, err
        }
        to[toIdx] = addr
    }

    subject, err := DoTemplate(r.SubjectTempl, values)
    if err != nil {
        return nil, err
    }

    message, err := DoTemplate(r.MessageTempl, values)
    if err != nil {
        return nil, err
    }
    return NewMail(from, to, subject, message)
}


// handleTemplate handles request for templated mails that are expanded
// on the server side. In addition to all parameters that are required
// for a simple plain e-mail, templated e-mails require 1 or more rows
// containing named values.
// The data has csv format, with the first row being the header and
// everything following being the substitute values.
//
// POST parameters:
// secret: 0 or 1 strings
// from: 1 string, must pass mail regex
// toTemplate: 1 or more strings, all must pass mail regex
// subjectTemplate: 1 string, no requirements
// messageTemplate: 1 string, no requirements
// data: 1 string, csv format (RFC 4180) with a header row
func handleTemplated(w http.ResponseWriter, r *http.Request) {
	handleWithReturn(w, func() (int, string) {
		/////////
		// POST only!
		if r.Method != "POST" {
			return 405, "405 method not allowed\n"
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

        templateRequest, err := TemplateFromStringMap(r.Form)
        if err != nil {
            return 400, "400 bad request (" + err.Error() + ")"
        }

        // delete all possible keys and check if there are any left
        delete(r.Form, "secret")
        delete(r.Form, "from")
        delete(r.Form, "toTemplate")
        delete(r.Form, "subjectTemplate")
        delete(r.Form, "messageTemplate")
        delete(r.Form, "records")

		// check for extra flags and fail if there are any
		if len(r.Form) > 0 {
			return 400, "400 bad request (extra arguments)"
		}

        for templateRequest.Next() {
            mail, err := templateRequest.GetMail()
            if err != nil {
                return 400, "400 bad request (" + err.Error() + ")"
            }

            log.Printf("from: '%s'", mail.From)
            log.Printf("to: '%s'", mail.To)
            log.Printf("subject: '%s'", mail.Subject)
            log.Printf("message:")
            for _, line := range strings.Split(mail.Message, "\n") {
                log.Printf(">%s", line)
            }
        }

		return 200, "200 ok\n"
	})
}

func main() {
	kingpin.Parse()

	if *secret == "" {
		log.Printf("CAUTION: NO SECRET REQUIRED, ABUSE LIKELY")
	}

	http.HandleFunc("/plain", handlePlain)
	http.HandleFunc("/template", handleTemplated)

	log.Printf("starting server...")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
