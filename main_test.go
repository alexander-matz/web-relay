package main;

import "testing"

func TestNewMailPositive(t *testing.T) {
    mail, err := NewMail("asdf@test.com", []string{"somebody@somewhere.com", "somebodyelse@a.com"}, "a subject", "a message")
    if err != nil {
        t.Fail()
    }
    if mail.From != "asdf@test.com" {
        t.Fail()
    }
    if mail.To[0] != "somebody@somewhere.com" {
        t.Fail()
    }
    if mail.To[1] != "somebodyelse@a.com" {
        t.Fail()
    }
    if mail.Subject != "a subject" {
        t.Fail()
    }
    if mail.Message != "a message" {
        t.Fail()
    }

}

func TestNewMailNegativeFrom(t *testing.T) {
    _, err := NewMail("asdf@", []string{"a@b.com"}, "a", "b")
    if err == nil {
        t.Fail()
    }
}

func TestNewMailNegativeTo(t *testing.T) {
    _, err := NewMail("asdf@asdf.com", []string{"a@asdf"}, "a", "b")
    if err == nil {
        t.Fail()
    }
    _, err = NewMail("asdf@asdf.com", []string{}, "a", "b")
    if err == nil {
        t.Fail()
    }
}

func TestNewMailNegativeSubject(t *testing.T) {
    _, err := NewMail("asdf@asdf.com", []string{"a@b.com"}, "", "b")
    if err == nil {
        t.Fail()
    }
}

func TestNewMailNegativeMessage(t *testing.T) {
    _, err := NewMail("asdf@asdf.com", []string{"a@b.com"}, "asdf", "")
    if err == nil {
        t.Fail()
    }
}

func TestMailFromStringMap(t *testing.T) {
    cases := []struct {
        input map[string][]string
        shouldFail bool
    }{
        {
            input: map[string][]string{
                "from": []string{"asdf@fdas.com"},
                "to": []string{"fdsa@fdsa.com"},
                "subject": []string{"subject"},
                "message": []string{"message"},
            },
            shouldFail: false,
        }, {
            input: map[string][]string{
                "from": []string{},
                "to": []string{"fdsa@fdsa.com"},
                "subject": []string{"subject"},
                "message": []string{"message"},
            },
            shouldFail: true,
        }, {
            input: map[string][]string{
                "from": []string{"fdsa@fdsa.com", "fdsa@fdsa.com"},
                "to": []string{"fdsa@fdsa.com"},
                "subject": []string{"subject"},
                "message": []string{"message"},
            },
            shouldFail: true,
        }, {
            input: map[string][]string{
                "from": []string{"fdsa@fdsa.com"},
                "to": []string{},
                "subject": []string{"subject"},
                "message": []string{"message"},
            },
            shouldFail: true,
        }, {
            input: map[string][]string{
                "from": []string{"fdsa@fdsa.com"},
                "to": []string{"fdsa@fdsa.com"},
                "subject": []string{""},
                "message": []string{"message"},
            },
            shouldFail: true,
        }, {
            input: map[string][]string{
                "from": []string{"fdsa@fdsa.com"},
                "to": []string{"fdsa@fdsa.com"},
                "subject": []string{"subject"},
                "message": []string{""},
            },
            shouldFail: true,
        },
    }
    for i, testCase := range(cases) {
        _, err := MailFromStringMap(testCase.input);
        if testCase.shouldFail {
            if err == nil {
                t.Logf("test case %d succeeded but should have failed", i)
                t.Fail()
            }
        } else {
            if err != nil {
                t.Logf("test case %d failed but should have succeeded", i)
                t.Fail()
            }
        }
    }
}

func TestDoTemplate(t *testing.T) {
    var vars = map[string]string {
        "asdf": "fdsa",
        "bsdf": "fdsb",
    };

    res, err := DoTemplate("--${asdf}${bsdf}--", vars);
    if err != nil || res != "--fdsafdsb--" {
        t.Fail()
    }

    _, err = DoTemplate("--${asdf}${csdf}--", vars);
    if err == nil {
        t.Fail()
    }
}
