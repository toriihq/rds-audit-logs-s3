package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"rdsauditlogss3/internal/entity"
	"io"
	"strings"
	"time"
	"strconv"
)

type AuditLogParser struct {
}

func NewAuditLogParser() *AuditLogParser {
	return &AuditLogParser{}
}

func (p *AuditLogParser) ParseEntries(data io.Reader, logFileTimestamp int64) ([]*entity.LogEntry, error) {
	var entries []*entity.LogEntry
	var currentEntry *entity.LogEntry

	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		txt := scanner.Text()
		if txt == "" {
			continue
		}

		record := strings.Split(txt,",")

		if len(record) < 2 {
			return nil, fmt.Errorf("could not parse data")
		}

		unixMicro, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse time: %v", err)
		}
		ts := time.UnixMicro(unixMicro)

		newTS := entity.LogEntryTimestamp{
			Year:  ts.Year(),
			Month: int(ts.Month()),
			Day:   ts.Day(),
			Hour:  ts.Hour(),
		}

		if currentEntry != nil && currentEntry.Timestamp != newTS {
			entries = append(entries, currentEntry)
			currentEntry = nil
		}

		if currentEntry == nil {
			currentEntry = &entity.LogEntry{
				Timestamp:        newTS,
				LogLine:          new(bytes.Buffer),
				LogFileTimestamp: logFileTimestamp,
			}
		}

		currentEntry.LogLine.WriteString(txt)
		currentEntry.LogLine.WriteString("\n")
	}

	entries = append(entries, currentEntry)

	return entries, nil
}
