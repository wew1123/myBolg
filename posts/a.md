---
title: 并查集的实现
date: 2026-02-01
summary: 使用 Go 实现并查集，包含路径压缩与按大小合并。
---

# 并查集的实现

定义结构体

```go
type UnionFind struct {
	parent map[int]int
	size   map[int]int
}
```

初始化

```go
func NewUnionFind(elements []int) *UnionFind {
	// 初始化每个人的parent都是自己
	parent := make(map[int]int)
	size := make(map[int]int)
	for _, v := range elements {
		parent[v] = v
		size[v] = 1
	}
	return &UnionFind{
		parent: parent,
		size:   size,
	}
}
```

查找祖先

```go
// 查找祖先
func (uf *UnionFind) Find(x int) int {
	for x != uf.parent[x] {
		// x = uf.parent[x]
		uf.parent[x] = uf.Find(uf.parent[x]) // 路径压缩,每个节点都指向他的祖先节点
		x = uf.parent[x]
	}
	return x
}

```

合并x,y所在的集合

```go
// 合并x,y所在的集合
func (uf *UnionFind) Union(x, y int) {
	rootX := uf.Find(x)
	rootY := uf.Find(y)

	//确保rootX是大的集合的根
	if uf.size[rootX] < uf.size[rootY] {
		rootX, rootY = rootY, rootX
	}
	// ry的根节点的父节点指向x
	uf.parent[rootY] = rootX
	uf.size[rootX] += uf.size[rootY]
}
```



```go
// 判断连通性
func (uf *UnionFind) isConnected(x, y int) bool {
	return uf.Find(x) == uf.Find(y)
}

// 查看所在集合的大小
func (uf *UnionFind) GetSetSize(x int) int {
	return uf.size[uf.Find(x)]
}
```



```go
func NewElements(x ...int) []int {
	return x
}
func main() {
	// 新建一个elements 切片
	els := NewElements(1, 2, 3, 5, 6, 9)
	uf := NewUnionFind(els)

	uf.Union(1, 2)
	uf.Union(3, 5)
	uf.Union(6, 9)
	uf.Union(1, 3)

	fmt.Println(uf.GetSetSize(3))
	fmt.Println(uf.isConnected(3, 5))
}

```

