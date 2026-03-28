package analysis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/decks"
)

type Features struct {
	AvgCost            float64     `json:"avg_cost"`
	ManaCurve          map[int]int `json:"mana_curve"`
	MinionCount        int         `json:"minion_count"`
	SpellCount         int         `json:"spell_count"`
	WeaponCount        int         `json:"weapon_count"`
	EarlyCurveCount    int         `json:"early_curve_count"`
	TopHeavyCount      int         `json:"top_heavy_count"`
	DrawCount          int         `json:"draw_count"`
	DiscoverCount      int         `json:"discover_count"`
	SingleRemovalCount int         `json:"single_removal_count"`
	AoeCount           int         `json:"aoe_count"`
	HealCount          int         `json:"heal_count"`
	BurnCount          int         `json:"burn_count"`
	TauntCount         int         `json:"taunt_count"`
	TokenCount         int         `json:"token_count"`
	DeathrattleCount   int         `json:"deathrattle_count"`
	BattlecryCount     int         `json:"battlecry_count"`
	ManaCheatCount     int         `json:"mana_cheat_count"`
	ComboPieceCount    int         `json:"combo_piece_count"`
	EarlyGameScore     float64     `json:"early_game_score"`
	MidGameScore       float64     `json:"mid_game_score"`
	LateGameScore      float64     `json:"late_game_score"`
	CurveBalanceScore  float64     `json:"curve_balance_score"`
}

type Result struct {
	Archetype             string                      `json:"archetype"`
	Confidence            float64                     `json:"confidence"`
	ConfidenceReasons     []string                    `json:"confidence_reasons,omitempty"`
	Features              Features                    `json:"features"`
	FunctionalRoleSummary []FunctionalRoleSummaryItem `json:"functional_role_summary,omitempty"`
	Strengths             []string                    `json:"strengths"`
	Weaknesses            []string                    `json:"weaknesses"`
	StructuralTags        []string                    `json:"structural_tags,omitempty"`
	StructuralTagDetails  []StructuralTagDetail       `json:"structural_tag_details,omitempty"`
	PackageDetails        []PackageDetail             `json:"package_details,omitempty"`
	SuggestedAdds         []string                    `json:"suggested_adds,omitempty"`
	SuggestedCuts         []string                    `json:"suggested_cuts,omitempty"`
}

type StructuralTagDetail struct {
	Tag         string `json:"tag"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type FunctionalRoleSummaryItem struct {
	Role        string `json:"role"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	Explanation string `json:"explanation"`
}

type PackageDetail struct {
	Package     string `json:"package"`
	Parent      string `json:"parent,omitempty"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Slots       int    `json:"slots"`
	TargetMin   int    `json:"target_min,omitempty"`
	TargetMax   int    `json:"target_max,omitempty"`
	Explanation string `json:"explanation"`
}

type Analyzer struct{}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Analyze(parsed decks.ParseResult) Result {
	features := extractFeatures(parsed)
	archetype := classifyArchetype(features)
	tags := inferStructuralTags(features, archetype)
	tagDetails := explainStructuralTags(features, archetype, tags)
	packageDetails := inferPackageDetails(features, archetype, tags)
	roleSummary := summarizeFunctionalRoles(features)

	return Result{
		Archetype:             archetype,
		Confidence:            inferConfidence(parsed, features, archetype),
		ConfidenceReasons:     inferConfidenceReasons(parsed, features, archetype),
		Features:              features,
		FunctionalRoleSummary: roleSummary,
		Strengths:             inferStrengths(features),
		Weaknesses:            inferWeaknesses(features),
		StructuralTags:        tags,
		StructuralTagDetails:  tagDetails,
		PackageDetails:        packageDetails,
		SuggestedAdds:         inferSuggestedAdds(parsed, features, archetype, tags, packageDetails),
		SuggestedCuts:         inferSuggestedCuts(parsed, features, archetype, tags, packageDetails),
	}
}

func inferPackageDetails(features Features, archetype string, tags []string) []PackageDetail {
	details := make([]PackageDetail, 0, 16)

	appendIfMeaningful := func(item PackageDetail) {
		if item.Status == "" {
			return
		}
		details = append(details, item)
	}

	earlyBoardSlots := features.ManaCurve[1] + features.ManaCurve[2]
	refillSlots := features.DrawCount + features.DiscoverCount
	reactiveSlots := features.SingleRemovalCount + features.AoeCount + features.HealCount
	burnSlots := features.BurnCount
	latePayoffSlots := countCurveSlots(features.ManaCurve, 7)
	manaCheatSlots := features.ManaCheatCount
	earlyBoardMin, earlyBoardMax := targetRangeFor(archetype, "early_board_package")
	refillMin, refillMax := targetRangeFor(archetype, "refill_package")
	reactiveMin, reactiveMax := targetRangeFor(archetype, "reactive_package")
	burnMin, burnMax := targetRangeFor(archetype, "burn_package")
	latePayoffMin, latePayoffMax := targetRangeFor(archetype, "late_payoff_package")
	manaCheatMin, manaCheatMax := targetRangeFor(archetype, "mana_cheat_package")

	appendIfMeaningful(packageDetailForRange(
		"early_board_package",
		"",
		"Early board package",
		earlyBoardSlots,
		earlyBoardMin,
		earlyBoardMax,
		fmt.Sprintf("The deck currently shows %d proactive 1-2 cost slots for early board development.", earlyBoardSlots),
	))
	appendIfMeaningful(packageDetailForRange(
		"refill_package",
		"",
		"Refill package",
		refillSlots,
		refillMin,
		refillMax,
		fmt.Sprintf("The list currently has %d draw or discover slots to reload resources.", refillSlots),
	))
	appendIfMeaningful(packageDetailForRange(
		"reactive_package",
		"",
		"Reactive package",
		reactiveSlots,
		reactiveMin,
		reactiveMax,
		fmt.Sprintf("The deck currently commits %d slots to removal, AoE, or healing.", reactiveSlots),
	))
	appendIfMeaningful(packageDetailForRange(
		"burn_package",
		"",
		"Burn package",
		burnSlots,
		burnMin,
		burnMax,
		fmt.Sprintf("The list currently presents %d direct-damage slots.", burnSlots),
	))
	appendIfMeaningful(packageDetailForRange(
		"late_payoff_package",
		"",
		"Late payoff package",
		latePayoffSlots,
		latePayoffMin,
		latePayoffMax,
		fmt.Sprintf("The deck currently holds %d slots at 7+ mana dedicated to late payoffs.", latePayoffSlots),
	))
	appendIfMeaningful(packageDetailForRange(
		"mana_cheat_package",
		"",
		"Mana cheat package",
		manaCheatSlots,
		manaCheatMin,
		manaCheatMax,
		fmt.Sprintf("The list currently has %d mana-cheat or cost-reduction slots.", manaCheatSlots),
	))

	if hasStructuralTag(tags, "aggro_top_end_conflict") || (archetype != "Control" && earlyBoardSlots >= earlyBoardMin && latePayoffSlots > latePayoffMax) {
		details = append(details, PackageDetail{
			Package:     "early_pressure_vs_late_payoff",
			Label:       "Pressure vs late payoff tension",
			Status:      "conflict",
			Slots:       latePayoffSlots,
			Explanation: fmt.Sprintf("The aggro-leaning shell still commits %d slots to 7+ mana payoffs, so the late package is fighting the pressure-first game plan.", latePayoffSlots),
		})
	}

	appendFinePackageDetails(&details, archetype, "early_board_package", []packageMetric{
		{key: "one_drop_openers", label: "One-drop openers", slots: features.ManaCurve[1], explanation: fmt.Sprintf("The list currently has %d one-cost cards that can open development on turn one.", features.ManaCurve[1])},
		{key: "two_drop_bridges", label: "Two-drop bridges", slots: features.ManaCurve[2], explanation: fmt.Sprintf("The list currently carries %d two-cost bridge slots for curve continuity.", features.ManaCurve[2])},
	})
	appendFinePackageDetails(&details, archetype, "refill_package", []packageMetric{
		{key: "draw_reload", label: "Draw reload", slots: features.DrawCount, explanation: fmt.Sprintf("The deck currently shows %d direct draw slots.", features.DrawCount)},
		{key: "discover_reload", label: "Discover reload", slots: features.DiscoverCount, explanation: fmt.Sprintf("The deck currently shows %d discover slots that can extend resources.", features.DiscoverCount)},
	})
	appendFinePackageDetails(&details, archetype, "reactive_package", []packageMetric{
		{key: "spot_removal_suite", label: "Spot removal suite", slots: features.SingleRemovalCount, explanation: fmt.Sprintf("The deck currently commits %d slots to targeted removal.", features.SingleRemovalCount)},
		{key: "board_clear_suite", label: "Board clear suite", slots: features.AoeCount, explanation: fmt.Sprintf("The deck currently commits %d slots to multi-target clears.", features.AoeCount)},
		{key: "survivability_suite", label: "Survivability suite", slots: features.HealCount + features.TauntCount, explanation: fmt.Sprintf("The deck currently has %d survivability slots from healing and taunt effects.", features.HealCount+features.TauntCount)},
	})
	appendFinePackageDetails(&details, archetype, "burn_package", []packageMetric{
		{key: "burn_reach_suite", label: "Burn reach suite", slots: features.BurnCount, explanation: fmt.Sprintf("The list currently presents %d direct-damage reach slots.", features.BurnCount)},
		{key: "token_pressure_suite", label: "Token pressure suite", slots: features.TokenCount, explanation: fmt.Sprintf("The list currently shows %d summon-driven pressure slots.", features.TokenCount)},
	})
	appendFinePackageDetails(&details, archetype, "late_payoff_package", []packageMetric{
		{key: "battlecry_value_suite", label: "Battlecry value suite", slots: features.BattlecryCount, explanation: fmt.Sprintf("The deck currently has %d battlecry-centric value slots.", features.BattlecryCount)},
		{key: "deathrattle_value_suite", label: "Deathrattle value suite", slots: features.DeathrattleCount, explanation: fmt.Sprintf("The deck currently has %d deathrattle value slots.", features.DeathrattleCount)},
		{key: "top_end_finishers", label: "Top-end finishers", slots: latePayoffSlots, explanation: fmt.Sprintf("The deck currently holds %d true 7+ mana finishing slots.", latePayoffSlots)},
	})
	appendFinePackageDetails(&details, archetype, "mana_cheat_package", []packageMetric{
		{key: "mana_acceleration_suite", label: "Mana acceleration suite", slots: features.ManaCheatCount, explanation: fmt.Sprintf("The deck currently has %d mana acceleration or discount slots.", features.ManaCheatCount)},
		{key: "combo_setup_suite", label: "Combo setup suite", slots: features.ComboPieceCount, explanation: fmt.Sprintf("The deck currently has %d combo-marked setup slots.", features.ComboPieceCount)},
	})

	return details
}

type packageMetric struct {
	key         string
	label       string
	slots       int
	explanation string
}

func summarizeFunctionalRoles(features Features) []FunctionalRoleSummaryItem {
	items := []FunctionalRoleSummaryItem{
		functionalRoleItem("refill", "Refill", features.DrawCount+features.DiscoverCount, "Draw and discover effects give the deck ways to reload resources."),
		functionalRoleItem("reactive", "Reactive", features.SingleRemovalCount+features.AoeCount+features.HealCount, "Removal, AoE, and healing shape how well the deck can answer opposing pressure."),
		functionalRoleItem("pressure", "Pressure", features.BurnCount+features.TokenCount, "Burn and token-style effects help the deck convert board leads or reach from hand."),
		functionalRoleItem("defense", "Defense", features.TauntCount+features.HealCount, "Taunt and healing determine how well the deck can absorb aggressive starts."),
		functionalRoleItem("value", "Value", features.BattlecryCount+features.DeathrattleCount, "Battlecry and deathrattle effects often indicate incremental value packages."),
		functionalRoleItem("mana_cheat", "Mana Cheat", features.ManaCheatCount, "Cost reduction and mana acceleration effects change how quickly payoffs come online."),
		functionalRoleItem("combo", "Combo", features.ComboPieceCount, "Combo-tagged cards point to sequencing-dependent setup turns."),
	}

	out := make([]FunctionalRoleSummaryItem, 0, len(items))
	for _, item := range items {
		if item.Count <= 0 {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Role < out[j].Role
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > 6 {
		return out[:6]
	}
	return out
}

func functionalRoleItem(role string, label string, count int, explanation string) FunctionalRoleSummaryItem {
	return FunctionalRoleSummaryItem{
		Role:        role,
		Label:       label,
		Count:       count,
		Explanation: explanation,
	}
}

func packageDetailForRange(pkg string, parent string, label string, slots int, targetMin int, targetMax int, base string) PackageDetail {
	status := packageStatus(slots, targetMin, targetMax)
	if status == "" {
		return PackageDetail{}
	}

	explanation := base
	switch status {
	case "underbuilt":
		explanation = fmt.Sprintf("%s Target range is %d-%d slots, so this package is still underbuilt.", base, targetMin, targetMax)
	case "overbuilt":
		explanation = fmt.Sprintf("%s Target range is %d-%d slots, so this package is starting to crowd out the rest of the deck.", base, targetMin, targetMax)
	case "balanced":
		explanation = fmt.Sprintf("%s That sits inside the current %d-%d slot target.", base, targetMin, targetMax)
	}

	return PackageDetail{
		Package:     pkg,
		Parent:      parent,
		Label:       label,
		Status:      status,
		Slots:       slots,
		TargetMin:   targetMin,
		TargetMax:   targetMax,
		Explanation: explanation,
	}
}

func appendFinePackageDetails(details *[]PackageDetail, archetype string, parent string, metrics []packageMetric) {
	for _, metric := range metrics {
		targetMin, targetMax := targetRangeFor(archetype, metric.key)
		item := packageDetailForRange(metric.key, parent, metric.label, metric.slots, targetMin, targetMax, metric.explanation)
		if item.Status == "balanced" && metric.slots == 0 {
			continue
		}
		if item.Status == "" {
			continue
		}
		*details = append(*details, item)
	}
}

func targetRangeFor(archetype string, pkg string) (int, int) {
	switch archetype {
	case "Aggro":
		switch pkg {
		case "early_board_package":
			return 6, 10
		case "refill_package":
			return 3, 6
		case "reactive_package":
			return 2, 5
		case "burn_package":
			return 3, 8
		case "late_payoff_package":
			return 0, 4
		case "mana_cheat_package":
			return 0, 4
		case "one_drop_openers":
			return 2, 4
		case "two_drop_bridges":
			return 4, 6
		case "draw_reload":
			return 1, 3
		case "discover_reload":
			return 1, 3
		case "spot_removal_suite":
			return 1, 3
		case "board_clear_suite":
			return 0, 2
		case "survivability_suite":
			return 0, 3
		case "burn_reach_suite":
			return 3, 8
		case "token_pressure_suite":
			return 1, 4
		case "battlecry_value_suite":
			return 1, 4
		case "deathrattle_value_suite":
			return 0, 3
		case "top_end_finishers":
			return 0, 4
		case "mana_acceleration_suite":
			return 0, 3
		case "combo_setup_suite":
			return 0, 3
		}
	case "Control":
		switch pkg {
		case "early_board_package":
			return 0, 4
		case "refill_package":
			return 3, 7
		case "reactive_package":
			return 5, 10
		case "burn_package":
			return 0, 4
		case "late_payoff_package":
			return 5, 9
		case "mana_cheat_package":
			return 0, 4
		case "one_drop_openers":
			return 0, 2
		case "two_drop_bridges":
			return 0, 3
		case "draw_reload":
			return 2, 4
		case "discover_reload":
			return 1, 3
		case "spot_removal_suite":
			return 2, 5
		case "board_clear_suite":
			return 2, 4
		case "survivability_suite":
			return 2, 5
		case "burn_reach_suite":
			return 0, 2
		case "token_pressure_suite":
			return 0, 2
		case "battlecry_value_suite":
			return 2, 5
		case "deathrattle_value_suite":
			return 1, 4
		case "top_end_finishers":
			return 5, 9
		case "mana_acceleration_suite":
			return 0, 2
		case "combo_setup_suite":
			return 0, 3
		}
	default:
		switch pkg {
		case "early_board_package":
			return 4, 8
		case "refill_package":
			return 2, 5
		case "reactive_package":
			return 3, 7
		case "burn_package":
			return 1, 5
		case "late_payoff_package":
			return 2, 5
		case "mana_cheat_package":
			return 0, 4
		case "one_drop_openers":
			return 1, 3
		case "two_drop_bridges":
			return 2, 5
		case "draw_reload":
			return 1, 3
		case "discover_reload":
			return 1, 3
		case "spot_removal_suite":
			return 1, 4
		case "board_clear_suite":
			return 1, 3
		case "survivability_suite":
			return 1, 4
		case "burn_reach_suite":
			return 1, 4
		case "token_pressure_suite":
			return 0, 3
		case "battlecry_value_suite":
			return 1, 4
		case "deathrattle_value_suite":
			return 0, 3
		case "top_end_finishers":
			return 2, 5
		case "mana_acceleration_suite":
			return 0, 3
		case "combo_setup_suite":
			return 0, 3
		}
	}
	return 0, 0
}

func packageStatus(slots int, targetMin int, targetMax int) string {
	if targetMin == 0 && targetMax == 0 {
		return ""
	}
	if slots < targetMin {
		return "underbuilt"
	}
	if slots > targetMax {
		return "overbuilt"
	}
	return "balanced"
}

func inferStructuralTags(features Features, archetype string) []string {
	tags := make([]string, 0, 6)
	oneTwoMinions := features.ManaCurve[1] + features.ManaCurve[2]
	sevenPlusCount := 0
	for cost, count := range features.ManaCurve {
		if cost >= 7 {
			sevenPlusCount += count
		}
	}

	reactiveSpellLoad := features.SingleRemovalCount + features.AoeCount + features.HealCount + features.DiscoverCount
	if oneTwoMinions <= 4 {
		tags = append(tags, "thin_early_board")
	}
	if features.DrawCount == 0 && archetype == "Aggro" {
		tags = append(tags, "low_refill")
	} else if features.DrawCount <= 2 {
		tags = append(tags, "light_card_draw")
	}
	if features.EarlyCurveCount < 10 {
		tags = append(tags, "thin_early_curve")
	}
	if features.SpellCount <= 6 && features.SingleRemovalCount+features.AoeCount <= 2 {
		tags = append(tags, "light_reactive_package")
	}
	if archetype == "Control" && features.LateGameScore < 0.3 {
		tags = append(tags, "light_late_game_payoff")
	}
	if features.SpellCount >= 16 && features.MinionCount <= 12 && reactiveSpellLoad >= 8 {
		tags = append(tags, "reactive_spell_saturation")
	}
	if sevenPlusCount >= 12 {
		tags = append(tags, "heavy_top_end")
	}
	if features.TopHeavyCount >= 10 {
		tags = append(tags, "clunky_curve")
	} else if sevenPlusCount >= 6 && features.EarlyCurveCount <= 8 {
		tags = append(tags, "clunky_curve")
	}
	if archetype == "Aggro" && features.TopHeavyCount >= 6 {
		tags = append(tags, "aggro_top_end_conflict")
	} else if archetype != "Control" && sevenPlusCount >= 6 && features.EarlyCurveCount <= 8 {
		tags = append(tags, "aggro_top_end_conflict")
	}
	if archetype == "Control" && features.EarlyCurveCount >= 14 {
		tags = append(tags, "control_early_slot_overload")
	}
	return tags
}

func explainStructuralTags(features Features, archetype string, tags []string) []StructuralTagDetail {
	if len(tags) == 0 {
		return nil
	}

	details := make([]StructuralTagDetail, 0, len(tags))
	oneTwoMinions := features.ManaCurve[1] + features.ManaCurve[2]
	earlySlots := features.EarlyCurveCount
	lateSlots := countCurveSlots(features.ManaCurve, 7)
	reactiveSpellLoad := features.SingleRemovalCount + features.AoeCount + features.HealCount + features.DiscoverCount

	for _, tag := range tags {
		detail := StructuralTagDetail{Tag: tag}
		switch tag {
		case "thin_early_board":
			detail.Title = "Thin early board"
			detail.Explanation = fmt.Sprintf("Only %d 1-2 cost minion slots show up here, so the deck can miss its first board contest window.", oneTwoMinions)
		case "low_refill":
			detail.Title = "Low refill"
			detail.Explanation = fmt.Sprintf("This %s shell has no meaningful draw package, so it can empty its hand before the pressure closes the game.", strings.ToLower(archetype))
		case "light_card_draw":
			detail.Title = "Light card draw"
			detail.Explanation = fmt.Sprintf("The list only shows %d total draw slots, which makes key pieces harder to find on time.", features.DrawCount)
		case "thin_early_curve":
			detail.Title = "Thin early curve"
			detail.Explanation = fmt.Sprintf("Only %d of the 30 slots land on turns 1-3, so the opening sequence is less consistent than it should be.", earlySlots)
		case "light_reactive_package":
			detail.Title = "Light reactive package"
			detail.Explanation = fmt.Sprintf("The deck only presents %d removal-oriented slots, which can leave opposing boards unanswered.", features.SingleRemovalCount+features.AoeCount)
		case "light_late_game_payoff":
			detail.Title = "Light late-game payoff"
			detail.Explanation = "The control lean is clear, but too few slots are dedicated to closing once the game goes long."
		case "reactive_spell_saturation":
			detail.Title = "Reactive spell saturation"
			detail.Explanation = fmt.Sprintf("About %d slots are tied up in reactive spells and utility effects, which risks crowding out proactive pressure.", reactiveSpellLoad)
		case "heavy_top_end":
			detail.Title = "Heavy top end"
			detail.Explanation = fmt.Sprintf("The deck commits %d slots to 7+ cost cards, so payoff density is starting to squeeze the rest of the curve.", lateSlots)
		case "clunky_curve":
			detail.Title = "Clunky curve"
			detail.Explanation = fmt.Sprintf("%d cards sit at 7+ mana, which raises the odds of awkward early hands.", features.TopHeavyCount)
		case "aggro_top_end_conflict":
			detail.Title = "Aggro top-end conflict"
			detail.Explanation = "The aggressive shell is real, but too many late cards ask the deck to play a slower game than its opener supports."
		case "control_early_slot_overload":
			detail.Title = "Control early-slot overload"
			detail.Explanation = "This control shell devotes too many slots to low-cost cards that do not naturally scale into the late game."
		default:
			detail.Title = strings.ReplaceAll(tag, "_", " ")
			detail.Explanation = "This structural signal highlights a deck-shape imbalance worth reviewing."
		}
		details = append(details, detail)
	}

	return details
}

func inferSuggestedAdds(parsed decks.ParseResult, features Features, archetype string, tags []string, packageDetails []PackageDetail) []string {
	adds := make([]string, 0, 3)
	oneTwoMinions := features.ManaCurve[1] + features.ManaCurve[2]
	drawSlots := features.DrawCount + features.DiscoverCount
	earlySlots := features.EarlyCurveCount
	packages := indexPackageDetails(packageDetails)

	if detail, ok := packages["refill_package"]; ok && detail.Status == "underbuilt" {
		adds = append(adds, fmt.Sprintf("%s is only %d slots right now. Push it toward %d-%d slots by converting low-priority midgame slots into draw or discover tools.", detail.Label, detail.Slots, detail.TargetMin, detail.TargetMax))
	}

	if detail, ok := packages["early_board_package"]; ok && detail.Status == "underbuilt" {
		adds = append(adds, fmt.Sprintf("%s is sitting at %d slots. Add 2-4 proactive 1-2 cost minions so the deck gets closer to the %d-%d slot target.", detail.Label, detail.Slots, detail.TargetMin, detail.TargetMax))
	}

	if detail, ok := packages["reactive_package"]; ok && detail.Status == "underbuilt" {
		adds = append(adds, fmt.Sprintf("%s only covers %d slots. Add 1-2 flexible interaction cards so the deck moves closer to the %d-%d slot range.", detail.Label, detail.Slots, detail.TargetMin, detail.TargetMax))
	}

	if detail, ok := packages["late_payoff_package"]; ok && detail.Status == "underbuilt" {
		adds = append(adds, fmt.Sprintf("%s only has %d slots. Add a cleaner 2-3 card finisher package so this %s shell can close once it stabilizes.", detail.Label, detail.Slots, strings.ToLower(archetype)))
	}

	if len(adds) == 0 {
		if hasStructuralTag(tags, "low_refill") {
			adds = append(adds, fmt.Sprintf("Convert 2-4 low-priority midgame slots into draw or discover tools; this list currently shows only %d refill slots.", drawSlots))
		} else if hasStructuralTag(tags, "light_card_draw") {
			adds = append(adds, fmt.Sprintf("Add 1-2 more draw slots so the deck can find its key turns more consistently; right now it only has %d refill slots.", drawSlots))
		}

		if hasStructuralTag(tags, "thin_early_board") {
			adds = append(adds, fmt.Sprintf("You only have %d one- and two-cost minion slots. Add 2-4 proactive openers so the deck can contest board before turn three.", oneTwoMinions))
		}

		if hasStructuralTag(tags, "thin_early_curve") && archetype != "Control" {
			adds = append(adds, fmt.Sprintf("Only %d of 30 slots land on turns 1-3. Add 2-3 cheaper bridge cards so the early opener stops skipping development turns.", earlySlots))
		}

		if hasStructuralTag(tags, "light_reactive_package") {
			adds = append(adds, fmt.Sprintf("The list only shows %d removal slots. Add 1-2 flexible interaction cards so opposing boards are easier to stabilize against.", features.SingleRemovalCount+features.AoeCount))
		}

		if hasStructuralTag(tags, "light_late_game_payoff") {
			adds = append(adds, fmt.Sprintf("Add a clearer 2-3 card payoff package at the top end so this %s shell has a reliable way to close after stabilizing.", strings.ToLower(archetype)))
		}
	}

	return limitSuggestions(adds)
}

func inferSuggestedCuts(parsed decks.ParseResult, features Features, archetype string, tags []string, packageDetails []PackageDetail) []string {
	cuts := make([]string, 0, 3)
	topEndNames := summarizeCardNames(selectCardsByCost(parsed.Cards, 7, 0), 3)
	reactiveNames := summarizeCardNames(selectReactiveCards(parsed.Cards), 3)
	lowImpactEarlyNames := summarizeCardNames(selectCardsByCost(parsed.Cards, 0, 2), 3)
	packages := indexPackageDetails(packageDetails)

	if hasStructuralTag(tags, "reactive_spell_saturation") {
		cuts = append(cuts, fmt.Sprintf("Reactive package pressure is too high here. Trim 2-3 narrower reactive spells such as %s if they are crowding out proactive threats.", reactiveNames))
	}

	if detail, ok := packages["late_payoff_package"]; ok && detail.Status == "overbuilt" {
		cuts = append(cuts, fmt.Sprintf("%s is up to %d slots, above the %d-%d target. Start trimming 2-4 of those 7+ cost top end slots, beginning with %s.", detail.Label, detail.Slots, detail.TargetMin, detail.TargetMax, topEndNames))
	}

	if detail, ok := packages["reactive_package"]; ok && detail.Status == "overbuilt" {
		cuts = append(cuts, fmt.Sprintf("%s is overbuilt at %d slots. Trim narrower reactive spells such as %s if they are crowding out threats.", detail.Label, detail.Slots, reactiveNames))
	}

	if detail, ok := packages["early_pressure_vs_late_payoff"]; ok && detail.Status == "conflict" {
		cuts = append(cuts, fmt.Sprintf("%s is creating real pressure tension. Cut some of the late package, especially %s, so the deck can stay committed to its faster plan.", detail.Label, topEndNames))
	}

	if len(cuts) == 0 {
		if hasStructuralTag(tags, "reactive_spell_saturation") {
			cuts = append(cuts, fmt.Sprintf("Trim 2-3 narrower reactive spells such as %s if they are crowding out proactive threats.", reactiveNames))
		}

		if hasStructuralTag(tags, "heavy_top_end") {
			cuts = append(cuts, fmt.Sprintf("Trim 2-4 of your 7+ cost slots, starting with cards like %s, so payoff density does not crowd out the turns before them.", topEndNames))
		}

		if hasStructuralTag(tags, "clunky_curve") {
			cuts = append(cuts, fmt.Sprintf("Cut a few high-cost cards from the top end, especially %s, so the opening turns become less clunky.", topEndNames))
		}

		if hasStructuralTag(tags, "aggro_top_end_conflict") {
			cuts = append(cuts, fmt.Sprintf("This aggro shell should trim slower finishers such as %s and turn those copies into pressure or refill.", topEndNames))
		}

		if hasStructuralTag(tags, "control_early_slot_overload") {
			cuts = append(cuts, fmt.Sprintf("Cut a few low-impact early slots such as %s if they do not scale into the slower control game plan.", lowImpactEarlyNames))
		}
	}

	return limitSuggestions(cuts)
}

func indexPackageDetails(items []PackageDetail) map[string]PackageDetail {
	index := make(map[string]PackageDetail, len(items))
	for _, item := range items {
		index[item.Package] = item
	}
	return index
}

func hasStructuralTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func limitSuggestions(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	if len(items) > 3 {
		return items[:3]
	}
	return items
}

func extractFeatures(parsed decks.ParseResult) Features {
	features := Features{
		ManaCurve: make(map[int]int),
	}

	totalCards := 0
	totalCost := 0

	for _, card := range parsed.Cards {
		totalCards += card.Count
		totalCost += card.Cost * card.Count
		features.ManaCurve[card.Cost] += card.Count

		switch {
		case card.Cost <= 3:
			features.EarlyCurveCount += card.Count
			features.EarlyGameScore += float64(card.Count)
		case card.Cost <= 6:
			features.MidGameScore += float64(card.Count)
		default:
			features.TopHeavyCount += card.Count
			features.LateGameScore += float64(card.Count)
		}

		switch card.CardType {
		case "MINION":
			features.MinionCount += card.Count
		case "SPELL":
			features.SpellCount += card.Count
		case "WEAPON":
			features.WeaponCount += card.Count
		}

		applyFunctionalSignals(&features, inferFunctionalRoles(card))
	}

	if totalCards > 0 {
		features.AvgCost = float64(totalCost) / float64(totalCards)
		features.EarlyGameScore = features.EarlyGameScore / float64(totalCards)
		features.MidGameScore = features.MidGameScore / float64(totalCards)
		features.LateGameScore = features.LateGameScore / float64(totalCards)
		features.CurveBalanceScore = curveBalanceScore(features.ManaCurve)
	}

	return features
}

type inferredFunctionalRoles struct {
	Draw          int
	Discover      int
	Heal          int
	Aoe           int
	SingleRemoval int
	Burn          int
	Taunt         int
	Deathrattle   int
	Battlecry     int
	Token         int
	ManaCheat     int
	Combo         int
}

func inferFunctionalRoles(card decks.DeckCard) inferredFunctionalRoles {
	roles := inferredFunctionalRoles{}
	roles = mergeInferredRoles(roles, inferredFunctionalRolesFromMetadata(card.Metadata, card.Count))
	roles = mergeInferredRoles(roles, inferredFunctionalRolesFromTags(card.FunctionalTags, card.Count))

	text := normalizeCardText(card.Text)
	if text != "" {
		roles = mergeInferredRoles(roles, inferredFunctionalRolesFromText(text, card.Count))
	}
	return roles
}

func mergeInferredRoles(base inferredFunctionalRoles, incoming inferredFunctionalRoles) inferredFunctionalRoles {
	base.Draw = maxRoleCount(base.Draw, incoming.Draw)
	base.Discover = maxRoleCount(base.Discover, incoming.Discover)
	base.Heal = maxRoleCount(base.Heal, incoming.Heal)
	base.Aoe = maxRoleCount(base.Aoe, incoming.Aoe)
	base.SingleRemoval = maxRoleCount(base.SingleRemoval, incoming.SingleRemoval)
	base.Burn = maxRoleCount(base.Burn, incoming.Burn)
	base.Taunt = maxRoleCount(base.Taunt, incoming.Taunt)
	base.Deathrattle = maxRoleCount(base.Deathrattle, incoming.Deathrattle)
	base.Battlecry = maxRoleCount(base.Battlecry, incoming.Battlecry)
	base.Token = maxRoleCount(base.Token, incoming.Token)
	base.ManaCheat = maxRoleCount(base.ManaCheat, incoming.ManaCheat)
	base.Combo = maxRoleCount(base.Combo, incoming.Combo)
	return base
}

func maxRoleCount(left int, right int) int {
	if right > left {
		return right
	}
	return left
}

func inferredFunctionalRolesFromMetadata(metadata cards.CardMetadata, count int) inferredFunctionalRoles {
	roles := inferredFunctionalRoles{}
	for _, item := range append(append([]string{}, metadata.Mechanics...), metadata.ReferencedTags...) {
		switch strings.ToUpper(strings.TrimSpace(item)) {
		case "DISCOVER":
			roles.Discover = count
		case "TAUNT":
			roles.Taunt = count
		case "DEATHRATTLE":
			roles.Deathrattle = count
		case "BATTLECRY":
			roles.Battlecry = count
		case "COMBO":
			roles.Combo = count
		}
	}
	return roles
}

func inferredFunctionalRolesFromText(text string, count int) inferredFunctionalRoles {
	roles := inferredFunctionalRoles{}
	if containsAny(text, "draw", "add a copy", "get a copy") {
		roles.Draw = count
	}
	if containsAny(text, "discover") {
		roles.Discover = count
	}
	if containsAny(text, "restore", "heal") {
		roles.Heal = count
	}
	if containsAny(text, "all enemy", "all minions", "all characters", "enemy board") {
		roles.Aoe = count
	}
	if containsSingleRemoval(text) {
		roles.SingleRemoval = count
	}
	if containsAny(text, "deal") && containsAny(text, "damage") {
		roles.Burn = count
	}
	if containsAny(text, "taunt") {
		roles.Taunt = count
	}
	if containsAny(text, "deathrattle") {
		roles.Deathrattle = count
	}
	if containsAny(text, "battlecry") {
		roles.Battlecry = count
	}
	if containsAny(text, "summon") {
		roles.Token = count
	}
	if containsManaCheat(text) {
		roles.ManaCheat = count
	}
	if containsAny(text, "if you played another card this turn", "combo:", "combo ") {
		roles.Combo = count
	}
	return roles
}

func inferredFunctionalRolesFromTags(tags []string, count int) inferredFunctionalRoles {
	roles := inferredFunctionalRoles{}
	for _, tag := range tags {
		switch strings.TrimSpace(tag) {
		case "draw":
			roles.Draw += count
		case "discover":
			roles.Discover += count
		case "heal":
			roles.Heal += count
		case "aoe":
			roles.Aoe += count
		case "single_removal":
			roles.SingleRemoval += count
		case "burn":
			roles.Burn += count
		case "taunt":
			roles.Taunt += count
		case "deathrattle":
			roles.Deathrattle += count
		case "battlecry":
			roles.Battlecry += count
		case "token":
			roles.Token += count
		case "mana_cheat":
			roles.ManaCheat += count
		case "combo":
			roles.Combo += count
		}
	}
	return roles
}

func applyFunctionalSignals(features *Features, roles inferredFunctionalRoles) {
	features.DrawCount += roles.Draw
	features.DiscoverCount += roles.Discover
	features.HealCount += roles.Heal
	features.AoeCount += roles.Aoe
	features.SingleRemovalCount += roles.SingleRemoval
	features.BurnCount += roles.Burn
	features.TauntCount += roles.Taunt
	features.DeathrattleCount += roles.Deathrattle
	features.BattlecryCount += roles.Battlecry
	features.TokenCount += roles.Token
	features.ManaCheatCount += roles.ManaCheat
	features.ComboPieceCount += roles.Combo
}

func normalizeCardText(text string) string {
	text = strings.ToLower(text)
	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		"!", " ",
		"?", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"\n", " ",
		"\r", " ",
		"'", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(text)), " ")
}

func containsAny(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, normalizeCardText(pattern)) {
			return true
		}
	}
	return false
}

func containsSingleRemoval(text string) bool {
	if containsAny(text, "destroy an enemy minion", "destroy a minion", "destroy target minion", "return an enemy minion", "return a minion") {
		return true
	}
	return containsAny(text, "deal") && containsAny(text, "to a minion", "to enemy minion", "to an enemy minion")
}

func containsManaCheat(text string) bool {
	return containsAny(text,
		"costs less",
		"costs",
		"reduce the cost",
		"reduce cost",
		"your next minion costs",
		"costs health instead",
		"mana cheat",
	) && containsAny(text, "cost", "costs", "reduce")
}

func countCurveSlots(curve map[int]int, minCost int) int {
	total := 0
	for cost, count := range curve {
		if cost >= minCost {
			total += count
		}
	}
	return total
}

func selectCardsByCost(cards []decks.DeckCard, minCost int, maxCost int) []decks.DeckCard {
	filtered := make([]decks.DeckCard, 0, len(cards))
	for _, card := range cards {
		if card.Cost < minCost {
			continue
		}
		if maxCost > 0 && card.Cost > maxCost {
			continue
		}
		filtered = append(filtered, card)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Cost == filtered[j].Cost {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Cost > filtered[j].Cost
	})
	return filtered
}

func selectReactiveCards(cards []decks.DeckCard) []decks.DeckCard {
	filtered := make([]decks.DeckCard, 0, len(cards))
	for _, card := range cards {
		if card.CardType != "SPELL" {
			continue
		}
		text := normalizeCardText(card.Text)
		if containsSingleRemoval(text) || containsAny(text, "discover", "restore", "heal", "all enemy", "all minions") {
			filtered = append(filtered, card)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Cost == filtered[j].Cost {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Cost < filtered[j].Cost
	})
	return filtered
}

func summarizeCardNames(cards []decks.DeckCard, limit int) string {
	if len(cards) == 0 {
		return strconv.Itoa(limit) + " flexible slots"
	}
	names := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, card := range cards {
		if _, ok := seen[card.Name]; ok {
			continue
		}
		seen[card.Name] = struct{}{}
		names = append(names, card.Name)
		if len(names) == limit {
			break
		}
	}
	if len(names) == 0 {
		return "your lowest-priority slots"
	}
	return strings.Join(names, ", ")
}

func classifyArchetype(features Features) string {
	lowCurve := features.ManaCurve[1] + features.ManaCurve[2] + features.ManaCurve[3]
	highCurve := 0
	for cost, count := range features.ManaCurve {
		if cost >= 5 {
			highCurve += count
		}
	}

	switch {
	case features.AvgCost <= 3 && lowCurve >= 12:
		return "Aggro"
	case features.AvgCost >= 5 && highCurve >= 12:
		return "Control"
	default:
		return "Midrange"
	}
}

func inferStrengths(features Features) []string {
	var strengths []string
	if features.ManaCurve[1]+features.ManaCurve[2] >= 10 {
		strengths = append(strengths, "Fast early curve")
	}
	if features.MinionCount >= 16 {
		strengths = append(strengths, "Consistent board development")
	}
	if features.AvgCost >= 5 {
		strengths = append(strengths, "Strong late-game profile")
	}
	if features.CurveBalanceScore >= 0.45 {
		strengths = append(strengths, "Well-spread mana curve")
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "Balanced card mix")
	}
	return strengths
}

func inferWeaknesses(features Features) []string {
	var weaknesses []string
	if features.AvgCost <= 3 {
		weaknesses = append(weaknesses, "May run out of resources")
	}
	if features.AvgCost >= 5 {
		weaknesses = append(weaknesses, "Slow early turns")
	}
	if features.TopHeavyCount >= 10 {
		weaknesses = append(weaknesses, "Top-heavy curve can create clunky early turns")
	}
	if features.SpellCount <= 6 {
		weaknesses = append(weaknesses, "Limited reactive spell package")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "No obvious structural weakness identified yet")
	}
	return weaknesses
}

func inferConfidence(parsed decks.ParseResult, features Features, archetype string) float64 {
	score := 0.35

	if parsed.TotalCount == 30 {
		score += 0.1
	}
	if parsed.Legality.Valid {
		score += 0.1
	}

	lowCurve := features.ManaCurve[1] + features.ManaCurve[2] + features.ManaCurve[3]
	highCurve := 0
	for cost, count := range features.ManaCurve {
		if cost >= 5 {
			highCurve += count
		}
	}

	switch archetype {
	case "Aggro":
		if features.AvgCost <= 3 {
			score += 0.1
		}
		if lowCurve >= 12 {
			score += 0.15
		}
		if features.EarlyGameScore >= 0.45 {
			score += 0.1
		}
	case "Control":
		if features.AvgCost >= 5 {
			score += 0.1
		}
		if highCurve >= 12 {
			score += 0.15
		}
		if features.LateGameScore >= 0.35 {
			score += 0.1
		}
	default:
		if features.AvgCost >= 3.2 && features.AvgCost <= 4.8 {
			score += 0.04
		}
		if features.CurveBalanceScore >= 0.45 {
			score += 0.04
		}
		if lowCurve >= 8 && highCurve >= 6 {
			score += 0.02
		}
	}

	functionalSignals := countNonZeroFunctionalSignals(features)
	if functionalSignals >= 4 {
		score += 0.1
	} else if functionalSignals >= 2 {
		score += 0.05
	}

	if archetype != "Midrange" && features.CurveBalanceScore >= 0.5 {
		score += 0.05
	}

	if score > 0.95 {
		return 0.95
	}
	if score < 0.1 {
		return 0.1
	}
	return score
}

func countNonZeroFunctionalSignals(features Features) int {
	count := 0
	for _, value := range []int{
		features.DrawCount,
		features.DiscoverCount,
		features.SingleRemovalCount,
		features.AoeCount,
		features.HealCount,
		features.BurnCount,
		features.TauntCount,
		features.TokenCount,
		features.DeathrattleCount,
		features.BattlecryCount,
		features.ManaCheatCount,
		features.ComboPieceCount,
	} {
		if value > 0 {
			count++
		}
	}
	return count
}

func inferConfidenceReasons(parsed decks.ParseResult, features Features, archetype string) []string {
	reasons := make([]string, 0, 4)

	lowCurve := features.ManaCurve[1] + features.ManaCurve[2] + features.ManaCurve[3]
	highCurve := 0
	for cost, count := range features.ManaCurve {
		if cost >= 5 {
			highCurve += count
		}
	}

	switch archetype {
	case "Aggro":
		if lowCurve >= 12 {
			reasons = append(reasons, "A dense low-curve package makes the aggressive read more reliable.")
		}
		if features.EarlyGameScore >= 0.45 {
			reasons = append(reasons, "A high early-game share reinforces fast pressure as the primary plan.")
		}
	case "Control":
		if highCurve >= 12 {
			reasons = append(reasons, "A heavy late-game concentration strongly supports a control profile.")
		}
		if features.LateGameScore >= 0.35 {
			reasons = append(reasons, "The deck commits a large share of slots to slower payoff turns.")
		}
	default:
		if features.CurveBalanceScore >= 0.45 {
			reasons = append(reasons, "The curve is spread across multiple turns, which fits a midrange shell.")
		}
		if lowCurve >= 8 && highCurve >= 6 {
			reasons = append(reasons, "Both early and late buckets are present, so the list does not lean hard into one extreme.")
		}
	}

	if functionalSignals := countNonZeroFunctionalSignals(features); functionalSignals >= 4 {
		reasons = append(reasons, "Multiple functional card signals give the structural read more support.")
	} else if functionalSignals == 0 {
		reasons = append(reasons, "Confidence is limited because card text signals are still sparse and mostly curve-driven.")
	}

	if !parsed.Legality.Valid || parsed.TotalCount != 30 {
		reasons = append(reasons, "Confidence is reduced because the submitted deck is not a clean, legal 30-card list.")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Confidence is driven mostly by general curve shape rather than one overwhelming archetype signal.")
	}

	if len(reasons) > 4 {
		return reasons[:4]
	}
	return reasons
}

func curveBalanceScore(curve map[int]int) float64 {
	if len(curve) == 0 {
		return 0
	}

	nonZeroBuckets := 0
	for _, count := range curve {
		if count > 0 {
			nonZeroBuckets++
		}
	}

	score := float64(nonZeroBuckets) / 7.0
	if score > 1 {
		return 1
	}
	return score
}
