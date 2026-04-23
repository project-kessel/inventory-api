package model

// TupleFilter represents filtering criteria for tuple queries.
// All fields are optional (nil = match any). Uses builder pattern for construction.
type TupleFilter struct {
	objectType   *ResourceType
	reporterType *ReporterType
	objectId     *LocalResourceId
	relation     *Relation
	subject      *TupleSubjectFilter
}

func NewTupleFilter() TupleFilter {
	return TupleFilter{}
}

func (f TupleFilter) WithObjectType(t ResourceType) TupleFilter       { f.objectType = &t; return f }
func (f TupleFilter) WithReporterType(t ReporterType) TupleFilter     { f.reporterType = &t; return f }
func (f TupleFilter) WithObjectId(id LocalResourceId) TupleFilter     { f.objectId = &id; return f }
func (f TupleFilter) WithRelation(r Relation) TupleFilter             { f.relation = &r; return f }
func (f TupleFilter) WithSubject(s TupleSubjectFilter) TupleFilter    { f.subject = &s; return f }

func (f TupleFilter) ObjectType() *ResourceType         { return f.objectType }
func (f TupleFilter) ReporterType() *ReporterType       { return f.reporterType }
func (f TupleFilter) ObjectId() *LocalResourceId        { return f.objectId }
func (f TupleFilter) Relation() *Relation               { return f.relation }
func (f TupleFilter) Subject() *TupleSubjectFilter      { return f.subject }

// TupleSubjectFilter represents subject filtering criteria within a TupleFilter.
// All fields are optional (nil = match any). Uses builder pattern for construction.
type TupleSubjectFilter struct {
	subjectType  *ResourceType
	reporterType *ReporterType
	subjectId    *LocalResourceId
	relation     *Relation
}

func NewTupleSubjectFilter() TupleSubjectFilter {
	return TupleSubjectFilter{}
}

func (f TupleSubjectFilter) WithSubjectType(t ResourceType) TupleSubjectFilter    { f.subjectType = &t; return f }
func (f TupleSubjectFilter) WithReporterType(t ReporterType) TupleSubjectFilter   { f.reporterType = &t; return f }
func (f TupleSubjectFilter) WithSubjectId(id LocalResourceId) TupleSubjectFilter  { f.subjectId = &id; return f }
func (f TupleSubjectFilter) WithRelation(r Relation) TupleSubjectFilter           { f.relation = &r; return f }

func (f TupleSubjectFilter) SubjectType() *ResourceType    { return f.subjectType }
func (f TupleSubjectFilter) ReporterType() *ReporterType   { return f.reporterType }
func (f TupleSubjectFilter) SubjectId() *LocalResourceId   { return f.subjectId }
func (f TupleSubjectFilter) Relation() *Relation           { return f.relation }
