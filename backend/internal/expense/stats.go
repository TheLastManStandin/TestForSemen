package expense

import (
	"context"
	"sort"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// StatsQuery selects data for the panel's KPI cards and charts.
type StatsQuery struct {
	From, To time.Time
	UserID   *int64
	Category *domain.Category
}

type CategoryTotal struct {
	Category domain.Category
	Total    domain.Money
	Count    int
}

type MemberTotal struct {
	UserID     int64
	MemberName string
	Total      domain.Money
	Count      int
}

type DayTotal struct {
	Date   time.Time
	UserID int64
	Total  domain.Money
}

// Stats aggregates a period and compares it to the previous period of equal length.
type Stats struct {
	Total      domain.Money
	PrevTotal  domain.Money
	Count      int
	ByCategory []CategoryTotal // total desc
	ByMember   []MemberTotal   // total desc
	ByDay      []DayTotal      // date asc, then user id asc
}

func (s *Service) Stats(ctx context.Context, q StatsQuery) (Stats, error) {
	if q.To.Before(q.From) {
		return Stats{}, domain.Invalid("to", "конец периода раньше начала")
	}
	rows, err := s.repo.Aggregate(ctx, AggQuery(q))
	if err != nil {
		return Stats{}, err
	}
	days := int(q.To.Sub(q.From).Hours()/24) + 1
	prevTo := q.From.AddDate(0, 0, -1)
	prevRows, err := s.repo.Aggregate(ctx, AggQuery{From: prevTo.AddDate(0, 0, -(days - 1)), To: prevTo, UserID: q.UserID, Category: q.Category})
	if err != nil {
		return Stats{}, err
	}

	type dayKey struct {
		date time.Time
		user int64
	}
	cats := map[domain.Category]CategoryTotal{}
	members := map[int64]MemberTotal{}
	byDay := map[dayKey]DayTotal{}
	st := Stats{}
	for _, r := range rows {
		st.Total += r.Total
		st.Count += r.Count

		c := cats[r.Category]
		c.Category, c.Total, c.Count = r.Category, c.Total+r.Total, c.Count+r.Count
		cats[r.Category] = c

		m := members[r.UserID]
		m.UserID, m.MemberName, m.Total, m.Count = r.UserID, r.MemberName, m.Total+r.Total, m.Count+r.Count
		members[r.UserID] = m

		k := dayKey{r.Date, r.UserID}
		d := byDay[k]
		d.Date, d.UserID, d.Total = r.Date, r.UserID, d.Total+r.Total
		byDay[k] = d
	}
	for _, r := range prevRows {
		st.PrevTotal += r.Total
	}

	st.ByCategory = make([]CategoryTotal, 0, len(cats))
	for _, c := range cats {
		st.ByCategory = append(st.ByCategory, c)
	}
	sort.Slice(st.ByCategory, func(i, j int) bool {
		if st.ByCategory[i].Total != st.ByCategory[j].Total {
			return st.ByCategory[i].Total > st.ByCategory[j].Total
		}
		return st.ByCategory[i].Category < st.ByCategory[j].Category
	})

	st.ByMember = make([]MemberTotal, 0, len(members))
	for _, m := range members {
		st.ByMember = append(st.ByMember, m)
	}
	sort.Slice(st.ByMember, func(i, j int) bool {
		if st.ByMember[i].Total != st.ByMember[j].Total {
			return st.ByMember[i].Total > st.ByMember[j].Total
		}
		return st.ByMember[i].UserID < st.ByMember[j].UserID
	})

	st.ByDay = make([]DayTotal, 0, len(byDay))
	for _, d := range byDay {
		st.ByDay = append(st.ByDay, d)
	}
	sort.Slice(st.ByDay, func(i, j int) bool {
		if !st.ByDay[i].Date.Equal(st.ByDay[j].Date) {
			return st.ByDay[i].Date.Before(st.ByDay[j].Date)
		}
		return st.ByDay[i].UserID < st.ByDay[j].UserID
	})
	return st, nil
}

// GroupBy selects how Summary groups totals.
type GroupBy string

const (
	GroupNone     GroupBy = "none"
	GroupCategory GroupBy = "category"
	GroupDay      GroupBy = "day"
)

// SummaryQuery is what the agent's get_summary tool asks for.
type SummaryQuery struct {
	From, To time.Time
	Category *domain.Category
	GroupBy  GroupBy
}

type SummaryGroup struct {
	Key   string
	Label string
	Total domain.Money
	Count int
}

type Summary struct {
	From, To time.Time
	Total    domain.Money
	Count    int
	Groups   []SummaryGroup // category: total desc; day: chronological; none: empty
}

func (s *Service) Summary(ctx context.Context, scope Scope, q SummaryQuery) (Summary, error) {
	switch q.GroupBy {
	case GroupNone, GroupCategory, GroupDay:
	default:
		return Summary{}, domain.Invalid("group_by", "ожидается none, category или day")
	}
	if q.To.Before(q.From) {
		return Summary{}, domain.Invalid("to", "конец периода раньше начала")
	}
	rows, err := s.repo.Aggregate(ctx, AggQuery{From: q.From, To: q.To, UserID: scope.UserID, Category: q.Category})
	if err != nil {
		return Summary{}, err
	}

	sum := Summary{From: q.From, To: q.To}
	groups := map[string]*SummaryGroup{}
	var order []string
	for _, r := range rows {
		sum.Total += r.Total
		sum.Count += r.Count

		var key, label string
		switch q.GroupBy {
		case GroupCategory:
			key, label = string(r.Category), r.Category.RU()
		case GroupDay:
			key, label = domain.FormatDate(r.Date), r.Date.Format("02.01")+" ("+domain.WeekdayRU(r.Date)+")"
		default:
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &SummaryGroup{Key: key, Label: label}
			groups[key] = g
			order = append(order, key) // rows are date-ordered, so day groups stay chronological
		}
		g.Total += r.Total
		g.Count += r.Count
	}
	for _, k := range order {
		sum.Groups = append(sum.Groups, *groups[k])
	}
	if q.GroupBy == GroupCategory {
		sort.SliceStable(sum.Groups, func(i, j int) bool { return sum.Groups[i].Total > sum.Groups[j].Total })
	}
	return sum, nil
}
