package db

const (
	VariableQueryAllPlayersName = "SELECT DISTINCT name FROM players ORDER BY name"
	VariableQueryLastReplayPlayersName = "SELECT name FROM players p JOIN replays r ON p.replay_id = r.id WHERE r.replay_date = (SELECT MAX(replay_date) FROM replays) ORDER BY name"
	VariableQueryLast50PlayersName = "SELECT DISTINCT name FROM ( SELECT p.name, r.replay_date FROM players p JOIN replays r ON p.replay_id = r.id ORDER BY r.replay_date DESC ) t LIMIT 50"
	VariableQueryRaces = "SELECT race FROM (SELECT 'Protoss' race UNION ALL SELECT 'Terran' race UNION ALL SELECT 'Zerg' race) t"
)
