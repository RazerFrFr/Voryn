package structs

import "encoding/xml"

type Presence struct {
	XMLName xml.Name `xml:"presence"`
	To      string   `xml:"to,attr"`
	From    string   `xml:"from,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Type    string   `xml:"type,attr,omitempty"`
	Show    string   `xml:"show,omitempty"`
	Status  string   `xml:"status,omitempty"`
}
