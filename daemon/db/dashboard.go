package db

type Dashboard struct {
	TodaysTasks       []Task
	OverdueTasks      []Task
	GoalsWithProgress []GoalWithProgress
	DomainHealth      []DomainHealthData
	DoneToday         []Task
}

func (db *DB) DashboardData() (*Dashboard, error) {
	todaysTasks, err := db.TodaysTasks()
	if err != nil {
		return nil, err
	}

	overdue, err := db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.status = 'pending' AND t.due_date < ? AND t.due_date IS NOT NULL`,
		today(),
	)
	if err != nil {
		return nil, err
	}

	goals, err := db.GoalsWithProgress(nil)
	if err != nil {
		return nil, err
	}

	health, err := db.DomainHealthAll()
	if err != nil {
		return nil, err
	}

	doneToday, err := db.queryTasks(
		`SELECT t.id, t.title, t.domain_id, t.goal_id, t.context, t.due_date, t.recurring, t.priority, t.status, t.micro_steps, t.notify, t.position, t.is_focus, t.created_at, t.completed_at, d.name
		 FROM tasks t LEFT JOIN domains d ON t.domain_id = d.id
		 WHERE t.status = 'done' AND t.completed_at >= ?`,
		today()+"T00:00:00Z",
	)
	if err != nil {
		return nil, err
	}

	if todaysTasks == nil {
		todaysTasks = []Task{}
	}
	if overdue == nil {
		overdue = []Task{}
	}
	if goals == nil {
		goals = []GoalWithProgress{}
	}
	if health == nil {
		health = []DomainHealthData{}
	}
	if doneToday == nil {
		doneToday = []Task{}
	}

	return &Dashboard{
		TodaysTasks:       todaysTasks,
		OverdueTasks:      overdue,
		GoalsWithProgress: goals,
		DomainHealth:      health,
		DoneToday:         doneToday,
	}, nil
}
