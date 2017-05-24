package sparse

import (
	"github.com/gonum/matrix"
	"github.com/gonum/matrix/mat64"
)

// compressedSparse represents the common structure for representing compressed sparse
// matrix formats e.g. CSR (Compressed Sparse Row) or CSC (Compressed Sparse Column)
type compressedSparse struct {
	i, j   int
	indptr []int
	ind    []int
	data   []float64
}

// NNZ returns the Number of Non Zero elements in the sparse matrix.
func (c *compressedSparse) NNZ() int {
	return len(c.data)
}

// at returns the element of the matrix located at coordinate i, j.  Depending upon the
// context and the type of compressed sparse (CSR or CSC) i and j could represent rows
// and columns or columns and rows respectively.
func (c *compressedSparse) at(i, j int) float64 {
	if uint(i) < 0 || uint(i) >= uint(c.i) {
		panic(matrix.ErrRowAccess)
	}
	if uint(j) < 0 || uint(j) >= uint(c.j) {
		panic(matrix.ErrColAccess)
	}

	// todo: consider a binary search if we can assume the data is ordered.
	for k := c.indptr[i]; k < c.indptr[i+1]; k++ {
		if c.ind[k] == j {
			return c.data[k]
		}
	}

	return 0
}

// CSR is a Compressed Sparse Row format sparse matrix implementation (sometimes called Compressed Row
// Storage (CRS) format) and implements the Matrix interface from gonum/matrix.  This allows large sparse
// (mostly zero values) matrices to be stored efficiently in memory (only storing non-zero values).
// CSR matrices are poor for constructing sparse matrices incrementally but very good for arithmetic operations.
// CSR, and their sibling CSC, matrices are similar to COOrdinate matrices except the row index slice is
// compressed.  Rather than storing the row indices of each non zero values (length == NNZ) each element, i, of the slice
// contains the cumulative count of non zero values in the matrix up to row i-1 of the matrix.  In this way,
// it is possible to address any element, i j, in the matrix with the following:
//
// 		for k := c.indptr[i]; k < c.indptr[i+1]; k++ {
//			if c.ind[k] == j {
//				return c.data[k]
//			}
//		}
//
// It should be clear that CSR is like CSC except the slices are row major order rather than column major and CSC
// is essentially the transpose of a CSR.
// As this type implements the gonum mat64.Matrix interface, it may be used with any of the Gonum mat64 functions that
// accept Matrix types as parameters in place of other matrix types included in the Gonum mat64 package e.g. mat64.Dense.
type CSR struct {
	compressedSparse
}

// NewCSR creates a new Compressed Sparse Row format sparse matrix.
// The matrix is initialised to the size of the specified r * c dimensions (rows * columns)
// with the specified slices containing row pointers and cols indexes of non-zero elements
// and the non-zero data values themselves respectively.  The supplied slices will be used as the
// backing storage to the matrix so changes to values of the slices will be reflected in the created matrix
// and vice versa.
func NewCSR(r int, c int, ia []int, ja []int, data []float64) *CSR {
	if uint(r) < 0 {
		panic(matrix.ErrRowAccess)
	}
	if uint(c) < 0 {
		panic(matrix.ErrColAccess)
	}

	return &CSR{
		compressedSparse: compressedSparse{
			i: r, j: c,
			indptr: ia,
			ind:    ja,
			data:   data,
		},
	}
}

// Dims returns the size of the matrix as the number of rows and columns
func (c *CSR) Dims() (int, int) {
	return c.i, c.j
}

// At returns the element of the matrix located at row i and column j.  At will panic if specified values
// for i or j fall outside the dimensions of the matrix.
func (c *CSR) At(m, n int) float64 {
	return c.at(m, n)
}

// T transposes the matrix creating a new CSC matrix sharing the same backing data but switching
// column and row sizes and index & index pointer slices i.e. rows become columns and columns become rows.
func (c *CSR) T() mat64.Matrix {
	return NewCSC(c.j, c.i, c.indptr, c.ind, c.data)
}

// ToDense returns a mat64.Dense dense format version of the matrix.  The returned mat64.Dense
// matrix will not share underlying storage with the receiver.
func (c *CSR) ToDense() *mat64.Dense {
	mat := mat64.NewDense(c.i, c.j, nil)

	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			mat.Set(i, c.ind[j], c.data[j])
		}
	}

	return mat
}

// ToDOK returns a DOK (Dictionary Of Keys) sparse format version of the matrix.  The returned DOK
// matrix will not share underlying storage with the receiver.
func (c *CSR) ToDOK() *DOK {
	dok := NewDOK(c.i, c.j)
	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			dok.Set(i, c.ind[j], c.data[j])
		}
	}

	return dok
}

// ToCOO returns a COOrdinate sparse format version of the matrix.  The returned COO matrix will
// share underlying storage with the receiver so any changes to either matrices will be reflected
// in the other.  NB this includes sorting the ordering of the non zero elements in the COO matrix
// e.g. for CSC conversion.
func (c *CSR) ToCOO() *COO {
	rows := make([]int, c.NNZ())

	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			rows[j] = i
		}
	}

	coo := NewCOO(c.i, c.j, rows, c.ind, c.data)

	return coo
}

// ToCSR returns the receiver
func (c *CSR) ToCSR() *CSR {
	return c
}

// ToCSC returns a Compressed Sparse Column sparse format version of the matrix.  The returned CSC matrix will
// share underlying storage with the receiver so any changes to either matrices will be reflected
// in the other.  NB, the current implementation uses COO as an intermediate format which changes the ordering
// of the data slice (from row major to col major) as part of the COO to CSC conversion.  This effectively
// corrupts the original CSR matrix.
func (c *CSR) ToCSC() *CSC {
	return c.ToCOO().ToCSC()
}

// ToType returns an alternative format version fo the matrix in the format specified.
func (c *CSR) ToType(matType MatrixType) mat64.Matrix {
	return matType.Convert(c)
}

// Mul takes the matrix product (Dot product) of the supplied matrices a and b and stores the result
// in the receiver.  If the number of columns does not equal the number of rows in b, Mul will panic.
func (c *CSR) Mul(a, b mat64.Matrix) {
	ar, ac := a.Dims()
	br, bc := b.Dims()

	if ac != br {
		panic(matrix.ErrShape)
	}

	if dia, ok := a.(*DIA); ok {
		c.mulDIA(dia, b, false)
		return
	}
	if dia, ok := b.(*DIA); ok {
		c.mulDIA(dia, a, true)
		return
	}

	c.indptr = make([]int, ar+1)

	c.i, c.j = ar, bc
	t := 0

	lhs, isCsr := a.(*CSR)

	if isCsr {
		for i := 0; i < ar; i++ {
			c.indptr[i] = t
			for j := 0; j < bc; j++ {
				var v float64
				// TODO Consider converting all Sparsers to CSR
				for k := lhs.indptr[i]; k < lhs.indptr[i+1]; k++ {
					v += lhs.data[k] * b.At(lhs.ind[k], j)
				}
				if v != 0 {
					t++
					c.ind = append(c.ind, j)
					c.data = append(c.data, v)
				}
			}
		}
	} else {
		row := make([]float64, ac)
		for i := 0; i < ar; i++ {
			c.indptr[i] = t
			for ci := range row {
				row[ci] = a.At(i, ci)
			}
			for j := 0; j < bc; j++ {
				var v float64
				for ci, e := range row {
					v += e * b.At(ci, j)
				}
				if v != 0 {
					t++
					c.ind = append(c.ind, j)
					c.data = append(c.data, v)
				}
			}
		}
	}

	c.indptr[c.i] = t
}

// mulDIA takes the matrix product of the diagonal matrix dia and an other matrix, other and stores the result
// in the receiver.  This method caters for the specialised case of multiplying by a diagonal matrix where significant
// optimisation is possible due to the sparsity pattern of the matrix.  If trans is true, the method will assume that
// other was the LHS (Left Hand Side) operand and that dia was the RHS.
func (c *CSR) mulDIA(dia *DIA, other mat64.Matrix, trans bool) {
	var csMat compressedSparse
	isCS := false

	if csr, ok := other.(*CSR); ok {
		// TODO consider converting all sparsers to CSR (for RHS operand)
		csMat = csr.compressedSparse
		isCS = true
		c.ind = make([]int, len(csMat.ind))
		c.data = make([]float64, len(csMat.data))
	}

	c.i, c.j = other.Dims()
	c.indptr = make([]int, c.i+1)
	t := 0
	raw := dia.Diagonal()

	for i := 0; i < c.i; i++ {
		c.indptr[i] = t
		var v float64

		if isCS {
			for k := csMat.indptr[i]; k < csMat.indptr[i+1]; k++ {
				var rawval float64
				if trans {
					rawval = raw[csMat.ind[k]]
				} else {
					rawval = raw[i]
				}
				v = csMat.data[k] * rawval
				if v != 0 {
					c.ind[t] = csMat.ind[k]
					c.data[t] = v
					t++
				}
			}
		} else {
			for k := 0; k < c.j; k++ {
				var rawval float64
				if trans {
					rawval = raw[k]
				} else {
					rawval = raw[i]
				}
				v = other.At(i, k) * rawval
				if v != 0 {
					c.ind = append(c.ind, k)
					c.data = append(c.data, v)
					t++
				}
			}
		}
	}

	c.indptr[c.i] = t

	return
}

// RowNNZ returns the Number of Non Zero values in the specified row i.  RowNNZ will panic if i is out of range.
func (c *CSR) RowNNZ(i int) int {
	if uint(i) < 0 || uint(i) >= uint(c.i) {
		panic(matrix.ErrRowAccess)
	}
	return c.indptr[i+1] - c.indptr[i]
}

// CSC is a Compressed Sparse Column format sparse matrix implementation (sometimes called Compressed Column
// Storage (CCS) format) and implements the Matrix interface from gonum/matrix.  This allows large sparse
// (mostly zero values) matrices to be stored efficiently in memory (only storing non-zero values).
// CSC matrices are poor for constructing sparse matrices incrementally but very good for arithmetic operations.
// CSC, and their sibling CSR, matrices are similar to COOrdinate matrices except the column index slice is
// compressed.  Rather than storing the column indices of each non zero values (length == NNZ) each element, i, of the slice
// contains the cumulative count of non zero values in the matrix up to column i-1 of the matrix.  In this way,
// it is possible to address any element, j i, in the matrix with the following:
//
// 		for k := c.indptr[i]; k < c.indptr[i+1]; k++ {
//			if c.ind[k] == j {
//				return c.data[k]
//			}
//		}
//
// It should be clear that CSC is like CSR except the slices are column major order rather than row major and CSC
// is essentially the transpose of a CSR.
// As this type implements the gonum mat64.Matrix interface, it may be used with any of the Gonum mat64 functions that
// accept Matrix types as parameters in place of other matrix types included in the Gonum mat64 package e.g. mat64.Dense.
type CSC struct {
	compressedSparse
}

// NewCSC creates a new Compressed Sparse Column format sparse matrix.
// The matrix is initialised to the size of the specified r * c dimensions (rows * columns)
// with the specified slices containing column pointers and row indexes of non-zero elements
// and the non-zero data values themselves respectively.  The supplied slices will be used as the
// backing storage to the matrix so changes to values of the slices will be reflected in the created matrix
// and vice versa.
func NewCSC(r int, c int, indptr []int, ind []int, data []float64) *CSC {
	if uint(r) < 0 {
		panic(matrix.ErrRowAccess)
	}
	if uint(c) < 0 {
		panic(matrix.ErrColAccess)
	}

	return &CSC{
		compressedSparse: compressedSparse{
			i: c, j: r,
			indptr: indptr,
			ind:    ind,
			data:   data,
		},
	}
}

// Dims returns the size of the matrix as the number of rows and columns
func (c *CSC) Dims() (int, int) {
	return c.j, c.i
}

// At returns the element of the matrix located at row i and column j.  At will panic if specified values
// for i or j fall outside the dimensions of the matrix.
func (c *CSC) At(m, n int) float64 {
	return c.at(n, m)
}

// T transposes the matrix creating a new CSR matrix sharing the same backing data but switching
// column and row sizes and index & index pointer slices i.e. rows become columns and columns become rows.
func (c *CSC) T() mat64.Matrix {
	return NewCSR(c.i, c.j, c.indptr, c.ind, c.data)
}

// ToDense returns a mat64.Dense dense format version of the matrix.  The returned mat64.Dense
// matrix will not share underlying storage with the receiver.
func (c *CSC) ToDense() *mat64.Dense {
	mat := mat64.NewDense(c.j, c.i, nil)

	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			mat.Set(c.ind[j], i, c.data[j])
		}
	}

	return mat
}

// ToDOK returns a DOK (Dictionary Of Keys) sparse format version of the matrix.  The returned DOK
// matrix will not share underlying storage with the receiver.
func (c *CSC) ToDOK() *DOK {
	dok := NewDOK(c.j, c.i)
	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			dok.Set(c.ind[j], i, c.data[j])
		}
	}

	return dok
}

// ToCOO returns a COOrdinate sparse format version of the matrix.  The returned COO matrix will
// share underlying storage with the receiver so any changes to either matrices will be reflected
// in the other.  NB this includes sorting the ordering of the non zero elements in the COO matrix
// e.g. for CSR conversion.
func (c *CSC) ToCOO() *COO {
	cols := make([]int, c.NNZ())

	for i := 0; i < len(c.indptr)-1; i++ {
		for j := c.indptr[i]; j < c.indptr[i+1]; j++ {
			cols[j] = i
		}
	}

	coo := NewCOO(c.j, c.i, c.ind, cols, c.data)

	return coo
}

// ToCSR returns a Compressed Sparse Row sparse format version of the matrix.  The returned CSR matrix will
// share underlying storage with the receiver so any changes to either matrices will be reflected
// in the other.  NB, the current implementation uses COO as an intermediate format which changes the ordering
// of the data slice (from col major to row major) as part of the COO to CSR conversion.  This effectively
// corrupts the original CSC matrix.
func (c *CSC) ToCSR() *CSR {
	return c.ToCOO().ToCSR()
}

// ToCSC returns the receiver
func (c *CSC) ToCSC() *CSC {
	return c
}

// ToType returns an alternative format version fo the matrix in the format specified.
func (c *CSC) ToType(matType MatrixType) mat64.Matrix {
	return matType.Convert(c)
}