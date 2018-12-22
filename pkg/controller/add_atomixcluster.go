package controller

import (
	"github.com/atomix/atomix-operator/pkg/controller/atomixcluster"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, atomixcluster.Add)
}
