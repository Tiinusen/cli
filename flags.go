package cli

type Flags map[string]interface{}

// Returns boolean flag, also returns false if flag is missing
func (f Flags) Boolean(name string) bool {
	if f == nil {
		return false
	}
	val, exists := f[name]
	if !exists {
		return false
	}
	return val.(bool)
}

// Returns integer flag, also returns 0 if flag is missing
func (f Flags) Integer(name string) int {
	if f == nil {
		return 0
	}
	val, exists := f[name]
	if !exists {
		return 0
	}
	return val.(int)
}

// Returns string flag, also returns 0 if flag is missing
func (f Flags) String(name string) string {
	if f == nil {
		return ""
	}
	val, exists := f[name]
	if !exists {
		return ""
	}
	return val.(string)
}
