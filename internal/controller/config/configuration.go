package config

type AmaltheaSessionConfiguration struct {
	ClusterType   ClusterType
	ImageRewriter ImageRewriter
}

type ImageRewriter interface {
	Rewrite(image string) (newImage string, err error)
}
