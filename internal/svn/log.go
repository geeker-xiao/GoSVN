package svn

import (
	"context"
	"encoding/xml"
	"strconv"
)

// Log returns commit history for path, parsed from `svn log --xml`.
// limit caps the number of revisions (0 = no limit); verbose adds changed
// paths per revision (svn log -v).
func (c *Client) Log(ctx context.Context, path string, limit int, verbose bool) ([]LogEntry, error) {
	args := []string{"log", "--xml"}
	if verbose {
		args = append(args, "-v")
	}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	args = append(args, path)
	data, err := c.runXML(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseLogXML(data)
}

type xmlLog struct {
	XMLName xml.Name     `xml:"log"`
	Entries []xmlLogEntry `xml:"logentry"`
}

type xmlLogEntry struct {
	Revision string      `xml:"revision,attr"`
	Author   string      `xml:"author"`
	Date     string      `xml:"date"`
	Msg      string      `xml:"msg"`
	Paths    *xmlPaths   `xml:"paths"`
}

type xmlPaths struct {
	Path []xmlLogPath `xml:"path"`
}

type xmlLogPath struct {
	Action   string `xml:"action,attr"`
	Kind     string `xml:"kind,attr"`
	PropMods string `xml:"prop-mods,attr"`
	TextMods string `xml:"text-mods,attr"`
	Value    string `xml:",chardata"`
}

func parseLogXML(data []byte) ([]LogEntry, error) {
	var doc xmlLog
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	entries := make([]LogEntry, 0, len(doc.Entries))
	for _, e := range doc.Entries {
		entry := LogEntry{
			Author:  e.Author,
			Date:    e.Date,
			Message: e.Msg,
		}
		if e.Revision != "" {
			if rev, err := strconv.Atoi(e.Revision); err == nil {
				entry.Revision = rev
			}
		}
		if e.Paths != nil {
			for _, p := range e.Paths.Path {
				entry.Paths = append(entry.Paths, LogPath{
					Action:   p.Action,
					Path:     p.Value,
					Kind:     p.Kind,
					TextMods: p.TextMods == "true",
					PropMods: p.PropMods == "true",
				})
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
