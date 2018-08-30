package resolver

type CachingResolver struct {
	Source Offerer
	Cache  CachingRequester
}

func (r CachingResolver) OfferV4ByMAC(info *ClientInfoV4, mac string) error {
	err := r.Source.OfferV4ByMAC(info, mac)
	if err != nil {
		// TODO log message
		return err
	}

	err = r.Cache.ReserveV4ByMAC(info, mac)
	if err != nil {
		// TODO log message
		return err
	}

	return nil
}

func (r CachingResolver) OfferV4ByID(info *ClientInfoV4, duid, iaid string) error {
	err := r.Source.OfferV4ByID(info, duid, iaid)
	if err != nil {
		// TODO log message
		return err
	}

	err = r.Cache.ReserveV4ByID(info, duid, iaid)
	if err != nil {
		// TODO log message
		return err
	}

	return nil
}

func (r CachingResolver) AcknowledgeV4ByMAC(info *ClientInfoV4, mac, ip string) error {
	err := r.Cache.AcknowledgeV4ByMAC(info, mac, ip)

	if err != nil {
		// TODO log message
		return err
	}

	return nil
}

func (r CachingResolver) AcknowledgeV4ByID(info *ClientInfoV4, duid, iaid, ip string) error {
	err := r.Cache.AcknowledgeV4ByID(info, duid, iaid, ip)

	if err != nil {
		// TODO log message
		return err
	}

	return nil
}
