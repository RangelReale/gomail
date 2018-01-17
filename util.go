package gomail

import "net/textproto"

// Creates a new header by copying the previous one
func CopyHeaders(h textproto.MIMEHeader) textproto.MIMEHeader {
	newh := make(textproto.MIMEHeader)
	for hn, hv := range h {
		newh[hn] = hv
	}
	return newh
}

// Creates a new header by copying the previous one, replacing some fields
func CopyHeadersReplace(h textproto.MIMEHeader, hreplace textproto.MIMEHeader) textproto.MIMEHeader {
	newh := make(textproto.MIMEHeader)
	for hn, hv := range h {
		newh[hn] = hv
	}
	for hn, hv := range hreplace {
		newh[hn] = hv
	}
	return newh
}

// Creates a new header by copying the previous one
func CopyOnlyHeaders(h textproto.MIMEHeader, list []string) textproto.MIMEHeader {
	newh := make(textproto.MIMEHeader)
	for _, hn := range list {
		if hv, ok := h[hn]; ok {
			newh[hn] = hv
		}
	}
	return newh
}
