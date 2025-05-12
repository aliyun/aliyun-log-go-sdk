package sls

import (
	"encoding/json"
	"errors"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
)

var (
	prefixRegex         = regexp.MustCompile(`^[a-zA-Z0-9_\-]+\.\*$`)
	suffixRegex         = regexp.MustCompile(`^\.\*[a-zA-Z0-9_\-]+$`)
	containsRegex       = regexp.MustCompile(`^\.\*[a-zA-Z0-9_\-]+\.\*$`) // except "+", e.g., ".+ABC.+" -> ABC
	literalExtractRegex = regexp.MustCompile(`[a-zA-Z0-9_\-]+`)
)

type MatchType int

func (m MatchType) String() string {
	switch m {
	case MatchEqual:
		return "EQUAL"
	case MatchPrefix:
		return "PREFIX"
	case MatchContains:
		return "CONTAINS"
	case MatchSuffix:
		return "SUFFIX"
	case MatchInclude:
		return "INCLUDE"
	case MatchRegexp:
		return "REGEXP"
	default:
		return "UNKNOWN"
	}
}

const (
	MatchEqual MatchType = iota
	MatchPrefix
	MatchContains
	MatchSuffix
	MatchInclude
	MatchRegexp
)

type Condition struct {
	MatchAll   bool // ".*" ".+"
	MatchType  MatchType
	MatchValue string

	IncludeValuesMap map[string]struct{}
	RegexPattern     *regexp.Regexp

	MatchFunc func(cond *Condition, val string) bool
}

func (c *Condition) Match(valueStr string) bool {
	if c.MatchAll {
		return true
	}
	return c.MatchFunc(c, valueStr)
}

func (c *Condition) String() string {
	if c.MatchAll {
		return "ALL"
	}

	matchType := c.MatchType
	var strBuilder strings.Builder
	strBuilder.WriteString(matchType.String())
	strBuilder.WriteByte(':')
	switch matchType {
	case MatchRegexp:
		strBuilder.WriteString(c.RegexPattern.String())
		strBuilder.WriteByte('#')
		strBuilder.WriteString(c.MatchValue)
		break
	case MatchInclude:
		for k, _ := range c.IncludeValuesMap {
			strBuilder.WriteString(k)
			strBuilder.WriteByte(',')
		}
		break
	default:
		strBuilder.WriteString(c.MatchValue)
	}
	return strBuilder.String()
}

func equalFunc(cond *Condition, val string) bool {
	return cond.MatchValue == val
}

func prefixFunc(cond *Condition, val string) bool {
	return strings.HasPrefix(val, cond.MatchValue)
}

func suffixFunc(cond *Condition, val string) bool {
	return strings.HasSuffix(val, cond.MatchValue)
}

func containsFunc(cond *Condition, val string) bool {
	return strings.Contains(val, cond.MatchValue)
}

func includeFunc(cond *Condition, val string) bool {
	_, ok := cond.IncludeValuesMap[val]
	return ok
}

func regexpFunc(cond *Condition, val string) bool {
	return cond.RegexPattern.MatchString(val)
}

func isPlainRegex(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags != syntax.Perl {
			return false
		}
		return true
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			if !isPlainRegex(sub) {
				return false
			}
		}
		if len(re.Sub) == 0 && re.Sub0[0] != nil {
			return isPlainRegex(re.Sub0[0])
		}
		return true
	default:
		return false
	}
}

func special(b byte) bool {
	_, ok := specialChars[b]
	return ok
}

var specialChars = map[byte]struct{}{
	'\\': {},
	'.':  {},
	'+':  {},
	'*':  {},
	'?':  {},
	'(':  {},
	')':  {},
	'|':  {},
	'[':  {},
	']':  {},
	'{':  {},
	'}':  {},
	'^':  {},
	'$':  {},
}

// unescapeRegexStr: delete all escape characters
func unescapeRegexStr(s string) string {
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}
	if i >= len(s) {
		return s
	}

	res := make([]byte, len(s))
	j := 0
	for idx := 0; idx < len(s); j++ {
		if s[idx] == '\\' && idx+1 < len(s) && special(s[idx+1]) {
			// delete escaped character: '\'
			res[j] = s[idx+1]
			idx += 2
			continue
		}
		res[j] = s[idx]
		idx++
	}

	return string(res[:j])
}

func isSimpleStrRegex(regexStr string) (bool, string) {
	re, err := syntax.Parse(regexStr, syntax.Perl)
	if err != nil {
		return false, ""
	}

	if !isPlainRegex(re) {
		return false, ""
	}

	str := unescapeRegexStr(regexStr)
	return true, str
}

func strictPrefixRegex(regexStr string) string {
	if prefixRegex.MatchString(regexStr) {
		return literalExtractRegex.FindStringSubmatch(regexStr)[0]
	}
	return ""
}

func strictSuffixRegex(regexStr string) string {
	if suffixRegex.MatchString(regexStr) {
		return literalExtractRegex.FindStringSubmatch(regexStr)[0]
	}
	return ""
}

func strictContainsRegex(regexStr string) string {
	if containsRegex.MatchString(regexStr) {
		return literalExtractRegex.FindStringSubmatch(regexStr)[0]
	}
	return ""
}

func transRegexToInclude(labelValue string) []string {
	valLen := len(labelValue)
	if valLen > 2 && labelValue[0] == '(' && labelValue[valLen-1] == ')' {
		labelValue = labelValue[1 : valLen-1]
	}
	splits := strings.Split(labelValue, "|")
	sort.Strings(splits)
	values := make([]string, 0, len(splits))
	for _, val := range splits {
		if val == "" {
			return nil
		}
		if ok, newVal := isSimpleStrRegex(val); ok {
			values = append(values, newVal)
			continue
		}
		return nil
	}
	return values
}

func checkIncludeCond(val string) []string {
	if !strings.Contains(val, "|") {
		return nil
	}
	return transRegexToInclude(val)
}

func generateMatchCondition(matchStr string) (*Condition, error) {
	if matchStr == "" {
		return nil, errors.New("empty match string")
	}
	if matchStr == ".*" || matchStr == ".+" {
		return &Condition{MatchAll: true, MatchType: MatchEqual, MatchValue: matchStr}, nil
	}
	if val := strictPrefixRegex(matchStr); val != "" {
		return &Condition{MatchAll: false, MatchType: MatchPrefix, MatchValue: val, MatchFunc: prefixFunc}, nil
	} else if val = strictContainsRegex(matchStr); val != "" {
		return &Condition{MatchAll: false, MatchType: MatchContains, MatchValue: val, MatchFunc: containsFunc}, nil
	} else if val = strictSuffixRegex(matchStr); val != "" {
		return &Condition{MatchAll: false, MatchType: MatchSuffix, MatchValue: val, MatchFunc: suffixFunc}, nil
	} else if includes := checkIncludeCond(matchStr); len(includes) > 0 {
		valuesMap := make(map[string]struct{}, len(includes))
		for _, v := range includes {
			valuesMap[v] = struct{}{}
		}
		return &Condition{MatchAll: false, MatchType: MatchInclude, MatchValue: matchStr, IncludeValuesMap: valuesMap, MatchFunc: includeFunc}, nil
	} else if ok, _ := isSimpleStrRegex(matchStr); ok {
		return &Condition{MatchAll: false, MatchType: MatchEqual, MatchValue: matchStr, MatchFunc: equalFunc}, nil
	} else {
		regex, cErr := regexp.Compile(matchStr)
		if cErr == nil {
			return &Condition{MatchAll: false, MatchType: MatchRegexp, MatchValue: matchStr, RegexPattern: regex, MatchFunc: regexpFunc}, nil
		}
		return nil, cErr
	}
}

func generateMatchConditions(matchStrs []string, matchConds []*Condition) []*Condition {
	for _, name := range matchStrs {
		newCond, err := generateMatchCondition(name)
		if err != nil {
			continue
		}
		if newCond.MatchAll {
			matchConds = matchConds[:0]
			matchConds = append(matchConds, newCond)
			break
		}
		matchConds = append(matchConds, newCond)
	}
	return matchConds
}

type ProjectStoreCond struct {
	ProjectCond     *Condition
	MetricStoreCond *Condition
}

func (p *ProjectStoreCond) String() string {
	return p.ProjectCond.String() + "__" + p.MetricStoreCond.String()
}

func (p *ProjectStoreCond) Match(project, store string) bool {
	// internal query 的projectName为空
	return (project == "" || p.ProjectCond.Match(project)) && p.MetricStoreCond.Match(store)
}

type StoreViewRoutingMatchStrategy struct {
	MetricNameConds   []*Condition
	ProjectStoreConds []*ProjectStoreCond
}

func (s *StoreViewRoutingMatchStrategy) String() string {
	var strBuilder strings.Builder
	for _, cond := range s.MetricNameConds {
		strBuilder.WriteString(cond.String())
		strBuilder.WriteByte('|')
	}
	strBuilder.WriteString(";;;")
	for _, cond := range s.ProjectStoreConds {
		strBuilder.WriteString(cond.ProjectCond.String())
		strBuilder.WriteString("_")
		strBuilder.WriteString(cond.MetricStoreCond.String())
		strBuilder.WriteByte('|')
	}
	return strBuilder.String()
}

type StoreViewRoutingStategiesItem struct {
	HashVal    uint64
	Strategies []*StoreViewRoutingMatchStrategy
}

type StoreViewRoutingConfigs []*MetricStoreViewRoutingConfig

func generateStoreviewRoutingStrategyOnConfigs(configsBuf []byte, newHashVal uint64) (*StoreViewRoutingStategiesItem, error) {
	strategyList := StoreViewRoutingConfigs{}
	err := json.Unmarshal(configsBuf, &strategyList)
	if err != nil {
		return nil, err
	}

	matchStrategies := make([]*StoreViewRoutingMatchStrategy, 0, len(strategyList))
	for _, strategy := range strategyList {
		matchStrategy := &StoreViewRoutingMatchStrategy{
			MetricNameConds:   make([]*Condition, 0, len(strategy.MetricNames)+1),
			ProjectStoreConds: make([]*ProjectStoreCond, 0, len(strategy.ProjectStores)),
		}
		// metric names
		if len(strategy.MetricNames) == 0 {
			matchStrategy.MetricNameConds = append(matchStrategy.MetricNameConds, &Condition{MatchAll: true, MatchType: MatchEqual})
		}
		matchStrategy.MetricNameConds = generateMatchConditions(strategy.MetricNames, matchStrategy.MetricNameConds)

		for _, projectStore := range strategy.ProjectStores {
			projCond, err := generateMatchCondition(projectStore.ProjectName)
			if err != nil {
				continue
			}
			storeCond, err := generateMatchCondition(projectStore.MetricStore)
			if err != nil {
				continue
			}
			matchStrategy.ProjectStoreConds = append(matchStrategy.ProjectStoreConds, &ProjectStoreCond{
				ProjectCond:     projCond,
				MetricStoreCond: storeCond,
			})
		}
		matchStrategies = append(matchStrategies, matchStrategy)
	}
	return &StoreViewRoutingStategiesItem{Strategies: matchStrategies, HashVal: newHashVal}, nil
}

type StoreViewRoutingChecker struct {
	strategyItem *StoreViewRoutingStategiesItem
}

func NewStoreViewRoutingChecker(routingConf []byte) (*StoreViewRoutingChecker, error) {
	strategyItem, err := generateStoreviewRoutingStrategyOnConfigs(routingConf, 0)
	if err != nil {
		return nil, err
	}
	return &StoreViewRoutingChecker{strategyItem: strategyItem}, nil
}

type MetricRoutingResult struct {
	MetricName    string
	ProjectStores []ProjectStore
}

func (s *StoreViewRoutingChecker) CheckPromQlQuery(query string, sourceProjects []ProjectStore) ([]MetricRoutingResult, error) {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return nil, err
	}

	dstProjects := make([]MetricRoutingResult, 0, 4)
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		switch n := node.(type) {
		case *parser.VectorSelector:
			metricNameOnEqual := ""
			for _, matcher := range n.LabelMatchers {
				if matcher.Name == "__name__" && matcher.Type == labels.MatchEqual {
					metricNameOnEqual = matcher.Value
					break
				}
			}
			if metricNameOnEqual == "" {
				// default: all stores matched
				dstProjects = append(dstProjects, MetricRoutingResult{MetricName: n.Name, ProjectStores: sourceProjects})
				return nil
			}
			matchedStrategies := make([]*StoreViewRoutingMatchStrategy, 0, len(s.strategyItem.Strategies))
			for _, strategy := range s.strategyItem.Strategies {
				metricNameMatched := false
				for _, metricNameCond := range strategy.MetricNameConds {
					if metricNameCond.Match(metricNameOnEqual) {
						metricNameMatched = true
						break
					}
				}
				if metricNameMatched {
					matchedStrategies = append(matchedStrategies, strategy)
				}
			}
			if (len(matchedStrategies)) == 0 {
				// default: all stores matched
				dstProjects = append(dstProjects, MetricRoutingResult{MetricName: n.Name, ProjectStores: sourceProjects})
				return nil
			}
			routingResult := MetricRoutingResult{MetricName: n.Name}
			for _, projectStore := range sourceProjects {
				storeMatched := false
				for _, strategy := range matchedStrategies {
					for _, storeCond := range strategy.ProjectStoreConds {
						if storeCond.Match(projectStore.ProjectName, projectStore.MetricStore) {
							storeMatched = true
							break
						}
					}
					if storeMatched {
						routingResult.ProjectStores = append(routingResult.ProjectStores, projectStore)
						break
					}
				}
			}
			dstProjects = append(dstProjects, routingResult)
		}
		return nil
	})

	return dstProjects, nil
}
