package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
)

type IndexCluster []int

type DendrogramStrata struct {
	Clusters []IndexCluster `json:"clusters"`
	Height   float64        `json:"height"`
}

// Performs hierarchical clustering using average linkage
// https://beginningwithml.wordpress.com/2019/04/17/11-3-hierarchical-clustering/
func HCluster(data []float64) [][]float64 {
	n := len(data)
	distMtx := NewMatrix2D(n, n)

	f, err := os.Create("clusters.txt")
	if err != nil {
		fmt.Printf("failed to create file\n")
		log.Panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	for i := 0; i < n*n; i++ {
		x := i % n
		y := i / n

		if x == y {
			distMtx.Set(i, math.Inf(1))
		} else {
			dist := math.Abs(data[x] - data[y])
			distMtx.Set(i, dist)
		}
	}

	// distMtx.Print()

	var dendrogram []DendrogramStrata

	clusters := make([]IndexCluster, n)
	for i := 0; i < n; i++ {
		clusters[i] = IndexCluster{i}
	}

	dendrogram = append(dendrogram, DendrogramStrata{clusters, 0})

	for iter := 0; iter < n-1; iter++ {
		closest, height := minDistance(distMtx)

		c1 := clusters[closest[0]]
		c2 := clusters[closest[1]]
		nextCluster := IndexCluster{}
		nextCluster = append(append(nextCluster, c1...), c2...)
		// fmt.Printf("%v\n", nextCluster)

		nextClusters := removeIndices(clusters, closest)
		nextClusters = append([]IndexCluster{nextCluster}, nextClusters...)
		// fmt.Printf("Clusters: %v\n", nextClusters)

		dendrogram = append(dendrogram, DendrogramStrata{nextClusters, height})
		clusters = nextClusters

		nextMtx := distMtx.RemoveRows(closest).RemoveCols(closest).AddRowAndCol()

		nextI := 0
		for i := 0; i < distMtx.Cols; i++ {
			if !slices.Contains(closest, i) {
				nextI++
				newDist := CompleteLink(distMtx, closest[0], closest[1], i) //, len(c1), len(c2))
				nextMtx.Set2D(nextI, 0, newDist)
				nextMtx.Set2D(0, nextI, newDist)
			}
		}

		// Ensure the value at [0,0] is not 0
		nextMtx.Set(0, math.Inf(1))
		distMtx = nextMtx
	}

	// fmt.Printf("\n%+v\n", dendrogram)
	dendrogramJson, err := json.Marshal(dendrogram)
	if err != nil {
		fmt.Printf("failed to marshal dendrogram to json\n")
		log.Panic(err)
	}
	w.Write(dendrogramJson)
	// fmt.Printf("Final clusters: %v\n", clusters)

	// diffIdx := 0
	// largestDiff := 0.0

	// for i := 1; i < len(dendrogram); i++ {
	// 	prev := dendrogram[i-1].Height
	// 	curr := dendrogram[i].Height
	// 	diff := curr - prev
	// 	if diff > largestDiff {
	// 		largestDiff = diff
	// 		diffIdx = i - 1
	// 	}
	// }

	// finalClusters := dendrogram[diffIdx].Clusters

	// priceClusters := make([][]float64, len(finalClusters))
	// for i, c := range finalClusters {
	// 	priceClusters[i] = make([]float64, len(c))
	// 	for j, idx := range c {
	// 		priceClusters[i][j] = data[idx]
	// 	}
	// }

	// Map the cluster indices back to their values in the source data
	mappedClusters := make([][][]float64, len(dendrogram))
	for i, s := range dendrogram {
		mappedClusters[i] = make([][]float64, len(s.Clusters))
		for j, c := range s.Clusters {
			mappedClusters[i][j] = make([]float64, len(c))
			for k, idx := range c {
				mappedClusters[i][j][k] = data[idx]
			}
		}
	}

	optimalCluster := analyzeClusters(mappedClusters)

	// fmt.Printf("Final clusters: %v\n", dendrogram[diffIdx].Clusters)
	return optimalCluster
}

func removeIndices[T any](s []T, is []int) []T {
	slices.Sort(is)
	slices.Reverse(is)
	ret := s
	for _, i := range is {
		next := make([]T, 0)
		next = append(next, ret[:i]...)
		next = append(next, ret[i+1:]...)
		ret = next
	}
	return ret
}

func minDistance(mat *Matrix2D) ([]int, float64) {
	minDistance := math.Inf(1)
	minPoints := make([]int, 2)
	for i := 0; i < mat.Rows*mat.Cols; i++ {
		x := i % mat.Cols
		y := i / mat.Cols

		dist := mat.Get(i)
		if dist < minDistance {
			minPoints[0], minPoints[1] = x, y
			minDistance = dist
		}
	}
	return minPoints, minDistance
}

// Lance-Williams update function to compute cluster distances at each step
func update(mat *Matrix2D, i, j, k int, alpha1, alpha2, beta, gamma float64) float64 {
	return alpha1*mat.Get2D(k, i) + alpha2*mat.Get2D(k, j) + beta*mat.Get2D(j, i) + gamma*math.Abs(mat.Get2D(k, i)-mat.Get2D(k, j))
}

func SingleLink(mat *Matrix2D, i, j, k int) float64 {
	return update(mat, i, j, k, 0.5, 0.5, 0, -0.5)
}

func CompleteLink(mat *Matrix2D, i, j, k int) float64 {
	return update(mat, i, j, k, 0.5, 0.5, 0, 0.5)
}

func AverageLink(mat *Matrix2D, i, j, k, ni, nj int) float64 {
	alpha1 := (float64(ni) / (float64(ni) + float64(nj)))
	alpha2 := (float64(nj) / (float64(ni) + float64(nj)))
	return update(mat, i, j, k, alpha1, alpha2, 0, 0)
}

func WardMethodLink(mat *Matrix2D, i, j, k, ni, nj, nk int) float64 {
	denom := (float64(ni) + float64(nj) + float64(nk))
	alpha1 := ((float64(ni) + float64(nj)) / denom)
	alpha2 := ((float64(nj) + float64(nk)) / denom)
	beta := -float64(nk) / denom
	return update(mat, i, j, k, alpha1, alpha2, beta, 0)
}

// Evaluates a dendrogram to find the optimal cut point that maximizes
// cluster validity (minimize intra-cluster variance while maintaining
// sufficient separation between clusters)
func analyzeClusters(clusters [][][]float64) [][]float64 {
	maxScore := math.Inf(-1)
	bestClusterIdx := -1
	scores := make([]float64, len(clusters))
	for i, clusters := range clusters {
		scores[i] = computeSilhouetteScore(clusters)
		if scores[i] > maxScore {
			maxScore = scores[i]
			bestClusterIdx = i
		}
	}

	return clusters[bestClusterIdx]
}

func computeSilhouetteScore(clusters [][]float64) float64 {
	var coefficients []float64
	// fmt.Printf("analyzing cluster group: %v\n", clusters)
	for i, c := range clusters {
		// scoresByCluster[i] = make([]float64, len(c))
		for j, point := range c {
			if len(c) == 1 {
				coefficients = append(coefficients, 0.0)
				continue
			}

			avgDist := 0.0
			for n_j, neighbor := range c {
				if j == n_j {
					continue
				}
				avgDist += math.Abs(neighbor - point)
			}
			avgDist /= float64(len(c)) - 1.0

			minClusterDist := math.Inf(1)
			for n_i, neighborCluster := range clusters {
				if n_i == i {
					continue
				}

				neighborDist := 0.0
				for _, neighbor := range neighborCluster {
					neighborDist += math.Abs(neighbor - point)
				}
				neighborDist /= float64(len(c))
				if minClusterDist > neighborDist {
					minClusterDist = neighborDist
				}
			}

			silhouetteScore := (minClusterDist - avgDist) / math.Max(minClusterDist, avgDist)
			coefficients = append(coefficients, silhouetteScore)
		}
	}

	meanScore := 0.0
	for _, coef := range coefficients {
		meanScore += coef
	}
	meanScore /= float64(len(coefficients))

	return meanScore
}

// func clusterKDE(data []float64) {
//   h := 3.0
//   fn := func (x float64) float64 {
//     return kernelDensityEstimator(x, h, data, gaussianKernel)
//   }

//   nSamples := 50
//   samples := make([]float64, nSamples)
//   for i := range samples {
//     t := float64(i) / float64(nSamples)
//   }
// }
// func lerp(start, end, t float64) float64 {
//   return start + ((end - start) * t)
// }

// func kernelDensityEstimator(x, h float64, data []float64, kernel func(x float64) float64) float64 {
// 	kernelSum := 0.0
// 	for _, xi := range data {
// 		kernelSum += kernel((x - xi) / h)
// 	}

// 	n := float64(len(data))
// 	return 1 / (n * h) * kernelSum
// }

// func gaussianKernel(x float64) float64 {
// 	return (1 / (math.Sqrt(2 * math.Pi))) * math.Exp(-(x*x)/2)
// }
