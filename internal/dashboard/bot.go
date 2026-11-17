package dashboard

// botGamesByPlayerKey degrades to no badges rather than failing the page.
func (d *Dashboard) botGamesByPlayerKey() map[string]int64 {
	m, err := d.dbStore.BotGamesByPlayerKey(d.ctx)
	if err != nil {
		return nil
	}
	return m
}
