package main

// store_plans.go — plans and their per-hash entries, reached only through these
// methods.
//
// A plan's header (name, scope, template, status, times) is a record the store
// keeps in memory on both engines. Its four maps — overrides, parked, mapping and
// satisfied, keyed by content hash — can hold an entry for every file in scope:
// on the JSON engine they stay on the record, and on the database engine they
// live only in the plan_entry table (catalogdb_drives.go), one row per hash. A
// change names only the entries it touches (PlanChange), so marking a file
// satisfied writes one row, not the plan. Every plan a method returns is the
// caller's own copy, without its maps; read them with PlanMaps.

import (
	"fmt"
	"sort"
	"time"
)

// PlanMaps is a copy of a plan's four maps. Every map is non-nil.
type PlanMaps struct {
	Overrides map[string]string
	Parked    map[string]bool
	Mapping   map[string]string
	Satisfied map[string]bool
}

func newPlanMaps() *PlanMaps {
	return &PlanMaps{Overrides: map[string]string{}, Parked: map[string]bool{}, Mapping: map[string]string{},
		Satisfied: map[string]bool{}}
}

// PlanChange is a change to a plan's maps. ClearMapping empties the mapping, and
// a non-nil Mapping replaces it. An empty override, and a false parked or
// satisfied mark, removes that hash's entry from that map.
type PlanChange struct {
	ClearMapping bool
	Mapping      map[string]string
	Overrides    map[string]string
	Parked       map[string]bool
	Satisfied    map[string]bool
}

// hashes is every hash the change names.
func (ch PlanChange) hashes() []string {
	var out []string
	for h := range ch.Mapping {
		out = append(out, h)
	}
	for h := range ch.Overrides {
		out = append(out, h)
	}
	for h := range ch.Parked {
		out = append(out, h)
	}
	for h := range ch.Satisfied {
		out = append(out, h)
	}
	return out
}

// apply makes the change to p's own maps (the JSON engine). A map left
// empty is nil, as it would be read back.
func (ch PlanChange) apply(p *Plan) {
	if ch.ClearMapping || ch.Mapping != nil {
		p.Mapping = nil
	}
	setS := func(m *map[string]string, h, v string) {
		if v == "" {
			delete(*m, h)
			return
		}
		if *m == nil {
			*m = map[string]string{}
		}
		(*m)[h] = v
	}
	setB := func(m *map[string]bool, h string, v bool) {
		if !v {
			delete(*m, h)
			return
		}
		if *m == nil {
			*m = map[string]bool{}
		}
		(*m)[h] = true
	}
	for h, v := range ch.Mapping {
		setS(&p.Mapping, h, v)
	}
	for h, v := range ch.Overrides {
		setS(&p.Overrides, h, v)
	}
	for h, v := range ch.Parked {
		setB(&p.Parked, h, v)
	}
	for h, v := range ch.Satisfied {
		setB(&p.Satisfied, h, v)
	}
	for _, m := range []*map[string]string{&p.Mapping, &p.Overrides} {
		if len(*m) == 0 {
			*m = nil
		}
	}
	for _, m := range []*map[string]bool{&p.Parked, &p.Satisfied} {
		if len(*m) == 0 {
			*m = nil
		}
	}
}

// headerOfPlan is a copy of a plan without its maps.
func headerOfPlan(p *Plan) *Plan {
	c := *p
	c.Overrides, c.Parked, c.Mapping, c.Satisfied = nil, nil, nil, nil
	if p.ArchiveIDs != nil {
		c.ArchiveIDs = append([]int{}, p.ArchiveIDs...)
	}
	if p.CompiledAt != nil {
		t := *p.CompiledAt
		c.CompiledAt = &t
	}
	if p.ClosedAt != nil {
		t := *p.ClosedAt
		c.ClosedAt = &t
	}
	return &c
}

func (s *Store) planLocked(id int) (*Plan, int) {
	for i, p := range s.c.Plans {
		if p.ID == id {
			return p, i
		}
	}
	return nil, -1
}

// AddPlanErr creates a plan from p, maps and all, giving it its ID, and returns a
// copy of its header.
func (s *Store) AddPlanErr(p *Plan) (*Plan, error) {
	ch := PlanChange{Mapping: p.Mapping, Overrides: p.Overrides, Parked: p.Parked, Satisfied: p.Satisfied}
	if err := checkPlanHashes(p.Name, ch); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := headerOfPlan(p)
	rec.ID = s.next("plan")
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	if rec.Status == "" {
		rec.Status = PlanDraft
	}
	if s.be != nil {
		s.dirty = true
		if err := s.be.planPut(rec, len(s.c.Plans), ch); err != nil {
			return nil, err
		}
	} else {
		ch.apply(rec)
	}
	s.c.Plans = append(s.c.Plans, rec)
	if err := s.save(); err != nil {
		return headerOfPlan(rec), err
	}
	return headerOfPlan(rec), nil
}

// Plans returns a copy of every plan's header, by ID.
func (s *Store) Plans() []*Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Plan, 0, len(s.c.Plans))
	for _, p := range s.c.Plans {
		out = append(out, headerOfPlan(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Plan returns a copy of a plan's header, or nil.
func (s *Store) Plan(id int) *Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, _ := s.planLocked(id); p != nil {
		return headerOfPlan(p)
	}
	return nil
}

// PlanMaps returns a copy of a plan's maps.
func (s *Store) PlanMaps(id int) (*PlanMaps, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _ := s.planLocked(id)
	if p == nil {
		return nil, fmt.Errorf("plan %d not found", id)
	}
	return s.planMapsLocked(p)
}

func (s *Store) planMapsLocked(p *Plan) (*PlanMaps, error) {
	if s.be != nil {
		return s.be.planEntries(p.ID)
	}
	m := newPlanMaps()
	for h, v := range p.Overrides {
		m.Overrides[h] = v
	}
	for h, v := range p.Parked {
		m.Parked[h] = v
	}
	for h, v := range p.Mapping {
		m.Mapping[h] = v
	}
	for h, v := range p.Satisfied {
		m.Satisfied[h] = v
	}
	return m, nil
}

// checkPlanHashes refuses a plan entry whose hash is not 64 lowercase
// hexadecimal characters, on both engines alike.
func checkPlanHashes(name string, ch PlanChange) error {
	for _, h := range ch.hashes() {
		if !canonicalHash(h) {
			return fmt.Errorf("plan %s: %q is not a content hash (64 lowercase hexadecimal characters)", name, h)
		}
	}
	return nil
}

// UpdatePlan saves p's header (everything but its maps).
func (s *Store) UpdatePlan(p *Plan) error { return s.ChangePlan(p, PlanChange{}) }

// ChangePlan saves p's header and the change ch to its maps, together.
func (s *Store) ChangePlan(p *Plan, ch PlanChange) error {
	if err := checkPlanHashes(p.Name, ch); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ord := s.planLocked(p.ID)
	if rec == nil {
		return fmt.Errorf("plan %d not found", p.ID)
	}
	hdr := headerOfPlan(p)
	if s.be != nil {
		s.dirty = true
		if err := s.be.planPut(hdr, ord, ch); err != nil {
			return err
		}
	} else {
		hdr.Overrides, hdr.Parked, hdr.Mapping, hdr.Satisfied = rec.Overrides, rec.Parked, rec.Mapping, rec.Satisfied
		ch.apply(hdr)
	}
	s.c.Plans[ord] = hdr
	return s.save()
}

// DeletePlan removes a plan and its entries.
func (s *Store) DeletePlan(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.c.Plans[:0]
	for _, p := range s.c.Plans {
		if p.ID != id {
			out = append(out, p)
		}
	}
	s.c.Plans = out
	return s.save()
}

// planHasSatisfiedLocked reports whether any hash of the plan is marked
// satisfied, or might be: a plan whose entries cannot be read counts as one.
func (s *Store) planHasSatisfiedLocked(p *Plan) bool {
	if s.be == nil {
		return len(p.Satisfied) > 0
	}
	has, err := s.be.planHasSatisfied(p.ID)
	return has || err != nil
}

// plansLocked is the catalog's plans, for Store methods kept outside the store's
// own files. Read the headers; change them only through Store methods.
func (s *Store) plansLocked() []*Plan { return s.c.Plans }
