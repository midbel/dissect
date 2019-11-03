package dissect

type boolean struct {
	Name  string
	Pos   int
	Raw   bool
	Value interface{}
}

type i8 struct {
	Name  string
	Pos   int
	Raw   int8
	Value interface{}
}

type u8 struct {
	Name  string
	Pos   int
	Raw   uint8
	Value interface{}
}

type i16 struct {
	Name  string
	Pos   int
	Raw   int16
	Value interface{}
}

type u16 struct {
	Name  string
	Pos   int
	Raw   uint16
	Value interface{}
}

type i32 struct {
	Name  string
	Pos   int
	Raw   int32
	Value interface{}
}

type u32 struct {
	Name  string
	Pos   int
	Raw   uint32
	Value interface{}
}

type i64 struct {
	Name  string
	Pos   int
	Raw   int64
	Value interface{}
}

type u64 struct {
	Name  string
	Pos   int
	Raw   uint64
	Value interface{}
}

type f32 struct {
	Name  string
	Pos   int
	Raw   float32
	Value interface{}
}

type f64 struct {
	Name  string
	Pos   int
	Raw   float64
	Value interface{}
}
