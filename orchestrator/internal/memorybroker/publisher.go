package memorybroker

// Publisher is the publish contract shared by existing producers.
type Publisher interface {
	Publish(topic string, payload any) error
}

// Ingestor is the non-blocking broker ingress contract.
type Ingestor interface {
	Ingest(topic string, payload any) bool
}

// TeePublisher mirrors published events to the ingestor while preserving upstream behavior.
type TeePublisher struct {
	upstream Publisher
	ingestor Ingestor
}

// NewTeePublisher creates a publisher that writes to upstream and ingestor.
func NewTeePublisher(upstream Publisher, ingestor Ingestor) *TeePublisher {
	return &TeePublisher{upstream: upstream, ingestor: ingestor}
}

// Publish forwards to upstream first, then mirrors to the ingestor best-effort.
func (p *TeePublisher) Publish(topic string, payload any) error {
	var err error
	if p.upstream != nil {
		err = p.upstream.Publish(topic, payload)
	}
	if p.ingestor != nil {
		_ = p.ingestor.Ingest(topic, payload)
	}
	return err
}
