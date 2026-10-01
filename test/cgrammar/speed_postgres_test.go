package cgrammar

import (
	"fmt"
	"strings"

	"github.com/xo/transit/grammars/postgres/postgres"
)

func init() {
	speedGrammars = append(speedGrammars, speedGrammar{"postgres", postgres.Language, postgres.Queries, speedPostgres})
}

// speedPostgres returns one SELECT statement of PostgreSQL with arithmetic,
// CASE expressions, function calls, casts, strings, a dollar-quoted string,
// which the external scanner reads, a join, a WHERE clause with IN and
// BETWEEN, GROUP BY and ORDER BY.
func speedPostgres(size int) string {
	var b strings.Builder
	b.WriteString("select\n")
	for i := 0; b.Len() < size-160; i++ {
		if i > 0 {
			b.WriteString(",\n")
		}
		switch i % 5 {
		case 0:
			fmt.Fprintf(&b, "  t.item_%d * %d + t.total as c_%d", i, i%9+1, i)
		case 1:
			fmt.Fprintf(&b, "  case when t.item_%d > %d and t.flag then t.item_%d else 0 end as c_%d", i, i, i, i)
		case 2:
			fmt.Fprintf(&b, "  coalesce(u.item_%d, 'item %d')::text as c_%d", i, i, i)
		case 3:
			fmt.Fprintf(&b, "  sum(t.item_%d) filter (where t.kind = $$k%d$$) as c_%d", i, i, i)
		case 4:
			fmt.Fprintf(&b, "  round(t.item_%d / 3.0, 2) as c_%d", i, i)
		}
	}
	b.WriteString("\nfrom items t\n  join users u on u.id = t.user_id\nwhere t.id in (1, 2, 3) and t.price between 10 and 20\ngroup by t.id, u.id\norder by t.id desc;\n")
	return b.String()
}
