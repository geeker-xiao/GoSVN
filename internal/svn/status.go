package svn

import (
	"context"
	"encoding/xml"
	"strconv"
)

// Status returns the working-copy status for path, parsed from
// `svn status --xml`.
func (c *Client) Status(ctx context.Context, path string) ([]StatusEntry, error) {
	data, err := c.runXML(ctx, "status", "--xml", path)
	if err != nil {
		return nil, err
	}
	return parseStatusXML(data)
}

// xmlStatus mirrors the shape of `svn status --xml` output.
type xmlStatus struct {
	XMLName xml.Name      `xml:"status"`
	Targets []xmlTarget   `xml:"target"`
}

type xmlTarget struct {
	Path   string      `xml:"path,attr"`
	Entry  []xmlEntry  `xml:"entry"`
}

type xmlEntry struct {
	Path     string       `xml:"path,attr"`
	WCStatus xmlWCStatus  `xml:"wc-status"`
}

type xmlWCStatus struct {
	Item     string      `xml:"item,attr"`
	Props    string      `xml:"props,attr"`
	Revision string      `xml:"revision,attr"`
	Commit   xmlCommit   `xml:"commit"`
}

type xmlCommit struct {
	Revision string `xml:"revision,attr"`
	Author   string `xml:"author"`
	Date     string `xml:"date"`
}

func parseStatusXML(data []byte) ([]StatusEntry, error) {
	var doc xmlStatus
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var entries []StatusEntry
	for _, t := range doc.Targets {
		for _, e := range t.Entry {
			entry := StatusEntry{
				Path:  e.Path,
				Item:  ItemStatus(e.WCStatus.Item),
				Props: e.WCStatus.Props,
			}
			if e.WCStatus.Revision != "" {
				if rev, err := strconv.Atoi(e.WCStatus.Revision); err == nil {
					entry.Revision = rev
				}
			}
			if e.WCStatus.Commit.Revision != "" {
				if rev, err := strconv.Atoi(e.WCStatus.Commit.Revision); err == nil {
					entry.Commit.Revision = rev
				}
			}
			entry.Commit.Author = e.WCStatus.Commit.Author
			entry.Commit.Date = e.WCStatus.Commit.Date
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
