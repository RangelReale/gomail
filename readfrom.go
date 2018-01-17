package gomail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
)

// ReadFrom implements io.ReadFrom. It parses a raw message into m.
func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	mw := &messageReader{r: r}
	mw.readMessage(m)
	return mw.n, mw.err
}

func (r *messageReader) readMessage(m *Message) {
	// clear previous message
	m.Reset()
	m.charset = "UTF-8"
	m.encoding = QuotedPrintable

	// reads a message
	var msg *mail.Message
	msg, r.err = mail.ReadMessage(r.r)
	if r.err != nil {
		return
	}

	// copy headers, except Content-Type and Mime-Version
	for hn, h := range msg.Header {
		hc := textproto.CanonicalMIMEHeaderKey(hn)
		switch hc {
		case "Content-Type", "Mime-Version":
			break
		default:
			m.header[hn] = h
		}
	}

	// parse parts
	var mediaType string
	var params map[string]string
	mediaType, params, r.err = mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if r.err != nil {
		return
	}

	// content type charset
	if charset, ok := params["charset"]; ok {
		m.charset = charset
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		// multipart
		boundary := ""
		if pboundary, ok := params["boundary"]; ok {
			boundary = pboundary
		}

		r.parseMultipart(m, msg.Body, mediaType, boundary)
		if r.err != nil {
			return
		}
	} else {
		// single body
		ps := []PartSetting{
			SetPartHeaders(CopyOnlyHeaders(textproto.MIMEHeader(msg.Header), []string{"Content-Type", "Content-Transfer-Encoding"})),
		}
		if pencoding := msg.Header.Get("Content-Transfer-Encoding"); pencoding != "" {
			ps = append(ps, SetPartEncoding(Encoding(pencoding)))
		}
		m.parts = []*part{m.newPart(mediaType, newReaderCopier(msg.Body), ps)}
	}
}

func (r *messageReader) parseMultipart(m *Message, mir io.Reader, mediaType string, boundary string) {
	// multipart
	mr := multipart.NewReader(mir, boundary)
	for {
		var p *multipart.Part
		p, r.err = mr.NextPart()
		if r.err == io.EOF {
			r.err = nil
			break
		}
		if r.err != nil {
			return
		}

		// parse part
		r.parsePart(m, p, mediaType)
		if r.err != nil {
			return
		}
	}
}

func (r *messageReader) parsePart(m *Message, part *multipart.Part, parentMediaType string) {
	var mediaType string
	var params map[string]string
	mediaType, params, r.err = mime.ParseMediaType(part.Header.Get("Content-Type"))
	if r.err != nil {
		return
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		// inner multipart
		boundary := ""
		if pboundary, ok := params["boundary"]; ok {
			boundary = pboundary
		}

		r.parseMultipart(m, part, mediaType, boundary)
		if r.err != nil {
			return
		}
	} else {
		// copy body bytes
		body := r.readPartBody(part, Encoding(part.Header.Get("Content-Transfer-Encoding")))
		if r.err != nil {
			return
		}

		if parentMediaType == "multipart/alternative" {
			// normal part
			ps := []PartSetting{
				SetPartHeaders(part.Header),
			}
			if pencoding := part.Header.Get("Content-Transfer-Encoding"); pencoding != "" {
				ps = append(ps, SetPartEncoding(Encoding(pencoding)))
			}

			m.parts = append(m.parts, m.newPart(mediaType, newReaderCopier(body), ps))
		} else {
			// attachment/embedded part

			// parse "name" from Content-Type as filename
			var filename string
			if pname, ok := params["name"]; ok {
				filename = pname
			}

			// is file or attachment?
			var contentDisposition string
			contentDisposition, params, r.err = mime.ParseMediaType(part.Header.Get("Content-Disposition"))
			if r.err != nil {
				return
			}

			// if Content-Disposition has filename, prefer it from Content-Type's name
			if pname, ok := params["filename"]; ok {
				filename = pname
			}

			// filename cannot be blank
			if strings.TrimSpace(filename) == "" {
				r.err = errors.New("Invalid blank file name")
				return
			}

			// add embedded/attach
			fs := []FileSetting{
				SetHeader(part.Header),
			}

			if contentDisposition == "inline" {
				m.embedded = m.appendFile(m.embedded, fileFromReader(filename, body), fs)
			} else {
				m.attachments = m.appendFile(m.attachments, fileFromReader(filename, body), fs)
			}
		}
	}
}

// Read a part body, decoding if needed
func (r *messageReader) readPartBody(part *multipart.Part, enc Encoding) io.Reader {
	var body bytes.Buffer
	if enc == Base64 {
		// decode base64
		_, r.err = body.ReadFrom(base64.NewDecoder(base64.StdEncoding, part))
	} else if enc == Unencoded || enc == QuotedPrintable || enc == "" {
		// multipart.Part already parses quoted-printable, and sets the header as blank
		_, r.err = body.ReadFrom(part)
	} else {
		r.err = fmt.Errorf("Unknown part encoding: %s", enc)
	}
	if r.err != nil {
		return nil
	}

	return &body
}

type messageReader struct {
	r          io.Reader
	n          int64
	partWriter io.Writer
	depth      uint8
	err        error
}
