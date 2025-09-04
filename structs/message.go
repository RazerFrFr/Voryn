package structs

import "encoding/xml"

type Message struct {
	XMLName xml.Name `xml:"message"`
	From    string   `xml:"from,attr"`
	To      string   `xml:"to,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Body    string   `xml:"body"`
}
