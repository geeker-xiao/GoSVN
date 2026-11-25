package svn

import (
	"context"
	"encoding/xml"
	"strconv"
)

// Info returns working-copy/repository metadata for path, parsed from
// `svn info --xml`.
func (c *Client) Info(ctx context.Context, path string) (*Info, error) {
	data, err := c.runXML(ctx, "info", "--xml", path)
	if err != nil {
		return nil, err
	}
	return parseInfoXML(data)
}

type xmlInfo struct {
	XMLName xml.Name    `xml:"info"`
	Entries []xmlInfoEntry `xml:"entry"`
}

type xmlInfoEntry struct {
	Path        string         `xml:"path,attr"`
	Kind        string         `xml:"kind,attr"`
	Revision    string         `xml:"revision,attr"`
	URL         string         `xml:"url"`
	RelativeURL string         `xml:"relative-url"`
	Repository  xmlRepository  `xml:"repository"`
	WCInfo      xmlWCInfo      `xml:"wc-info"`
	Commit      xmlCommit      `xml:"commit"`
}

type xmlRepository struct {
	Root string `xml:"root"`
	UUID string `xml:"uuid"`
}

type xmlWCInfo struct {
	WCRootAbspath string `xml:"wcroot-abspath"`
	Schedule      string `xml:"schedule"`
	Depth         string `xml:"depth"`
}

func parseInfoXML(data []byte) (*Info, error) {
	var doc xmlInfo
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Entries) == 0 {
		return &Info{}, nil
	}
	e := doc.Entries[0]
	info := &Info{
		Path:        e.Path,
		Kind:        e.Kind,
		URL:         e.URL,
		RelativeURL: e.RelativeURL,
		RepoRoot:    e.Repository.Root,
		RepoUUID:    e.Repository.UUID,
		WCRoot:      e.WCInfo.WCRootAbspath,
		Schedule:    e.WCInfo.Schedule,
		Depth:       e.WCInfo.Depth,
	}
	if e.Revision != "" {
		if rev, err := strconv.Atoi(e.Revision); err == nil {
			info.Revision = rev
		}
	}
	if e.Commit.Revision != "" {
		if rev, err := strconv.Atoi(e.Commit.Revision); err == nil {
			info.Commit.Revision = rev
		}
	}
	info.Commit.Author = e.Commit.Author
	info.Commit.Date = e.Commit.Date
	return info, nil
}
