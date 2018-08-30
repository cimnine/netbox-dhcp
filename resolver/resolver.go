package resolver

type CachingResolver struct {
	Source Offerer
	Cache  CachingRequester
}

func (r CachingResolver) OfferV4ByMAC(mac string) (*ClientInfoV4, error) {
	clientInfo, err := r.Source.OfferV4ByMAC(mac)
	if err != nil {
		// TODO log message
		return nil, err
	}

	err = r.Cache.ReserveV4ByMAC(mac, clientInfo)
	if err != nil {
		// TODO log message
		return nil, err
	}

	return clientInfo, nil
}

func (r CachingResolver) OfferV4ByID(duid, iaid string) (*ClientInfoV4, error) {
	clientInfo, err := r.Source.OfferV4ByID(duid, iaid)
	if err != nil {
		// TODO log message
		return nil, err
	}

	err = r.Cache.ReserveV4ByID(duid, iaid, clientInfo)
	if err != nil {
		// TODO log message
		return nil, err
	}

	return clientInfo, nil
}

func (r CachingResolver) AcknowledgeV4ByMAC(mac, ip string) (*ClientInfoV4, error) {
	clientInfo, err := r.Cache.AcknowledgeV4ByMAC(mac, ip)

	if err != nil {
		// TODO log message
		return nil, err
	}

	return clientInfo, nil
}

func (r CachingResolver) AcknowledgeV4ByID(duid, iaid, ip string) (*ClientInfoV4, error) {
	clientInfo, err := r.Cache.AcknowledgeV4ByID(duid, iaid, ip)

	if err != nil {
		// TODO log message
		return nil, err
	}

	return clientInfo, nil
}
