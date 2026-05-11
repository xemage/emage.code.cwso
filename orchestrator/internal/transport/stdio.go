package transport

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/emage/cwso/orchestrator/internal/logging"
)

// RunStdio reads newline-delimited JSON-RPC messages from stdin and writes
// responses to stdout. Logs are intentionally routed to stderr (set up in
// the logging package) to keep stdout pure JSON-RPC framing.
//
// stdio sessions are implicitly trusted: the role defaults to "orchestrator".
func RunStdio(ctx context.Context, log *logging.Logger, h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error)) error {
	in := bufio.NewReaderSize(os.Stdin, 1<<16)
	var outMu sync.Mutex
	sess := &Session{Role: "orchestrator", Subject: "stdio"}

	log.Info().Msg("stdio transport ready")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("stdio context cancelled")
			return nil
		default:
		}

		line, err := readLine(in)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Info().Msg("stdio EOF")
				return nil
			}
			return err
		}
		if len(line) == 0 {
			continue
		}

		resp, herr := h(ctx, sess, line)
		if herr != nil {
			log.Error().Err(herr).Msg("handler error")
			continue
		}
		if resp == nil {
			continue // notification
		}
		outMu.Lock()
		_, werr := os.Stdout.Write(append(resp, '\n'))
		outMu.Unlock()
		if werr != nil {
			log.Error().Err(werr).Msg("stdout write error")
			return werr
		}
	}
}

// readLine reads up to and including a newline, returning the line without it.
// 8 MiB cap to defend against unbounded input.
func readLine(r *bufio.Reader) ([]byte, error) {
	const maxLine = 8 << 20
	var line []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		if err != nil {
			return nil, err
		}
		line = append(line, chunk...)
		if len(line) > maxLine {
			return nil, errors.New("input line exceeds 8MiB limit")
		}
		if !isPrefix {
			return line, nil
		}
	}
}
