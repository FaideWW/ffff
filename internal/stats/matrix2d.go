package stats

import (
	"fmt"
	"slices"
)

type Matrix2D struct {
	Data []float64
	Rows int
	Cols int
}

func NewMatrix2D(m, n int) *Matrix2D {
	return &Matrix2D{
		Data: make([]float64, m*n),
		Rows: m,
		Cols: n,
	}
}

func (m *Matrix2D) Set2D(x, y int, v float64) {
	i := y*m.Cols + x
	m.Data[i] = v
}

func (m *Matrix2D) Get2D(x, y int) float64 {
	i := y*m.Cols + x
	return m.Data[i]
}

func (m *Matrix2D) Set(i int, v float64) {
	m.Data[i] = v
}

func (m *Matrix2D) Get(i int) float64 {
	return m.Data[i]
}

func (m *Matrix2D) Size() int {
	return m.Cols * m.Rows
}

func (m *Matrix2D) Print() {
	for y := 0; y < m.Rows; y++ {
		for x := 0; x < m.Cols; x++ {
			i := y*m.Cols + x
			fmt.Printf("%4.0f ", m.Data[i])
		}
		fmt.Printf("\n")
	}
}

func (m *Matrix2D) RemoveRows(ns []int) *Matrix2D {
	mNext := NewMatrix2D(m.Rows-len(ns), m.Cols)
	// Copy data into new matrix (skipping row n)
	iNext := 0
	for i := 0; i < m.Size(); i++ {
		y := i / m.Cols
		if !slices.Contains(ns, y) {
			mNext.Set(iNext, m.Get(i))
			iNext++
		}
	}

	return mNext
}

func (m *Matrix2D) RemoveCols(ns []int) *Matrix2D {
	mNext := NewMatrix2D(m.Rows, m.Cols-len(ns))

	iNext := 0
	for i := 0; i < m.Size(); i++ {
		x := i % m.Cols
		if !slices.Contains(ns, x) {

			mNext.Set(iNext, m.Get(i))
			iNext++
		}
	}

	return mNext
}

// Adds a row to the front of the matrix
func (m *Matrix2D) AddRowAndCol() *Matrix2D {
	mNext := NewMatrix2D(m.Rows+1, m.Cols+1)

	// Copy data into new matrix (skipping row n)
	for i := 0; i < m.Size(); i++ {
		y := i/m.Cols + 1
		x := i%m.Cols + 1
		mNext.Set2D(x, y, m.Get(i))
	}

	return mNext
}
