package dashboard

// botGamesByPlayerKey degrades to no badges rather than failing the page.
func (d *Dashboard) botGamesByPlayerKey() map[string]int64 {
	m, _ := d.dbStore.BotGamesByPlayerKey(d.ctx)
	return m
}
