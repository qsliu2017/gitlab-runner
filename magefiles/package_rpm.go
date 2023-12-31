// Code generated by mage package:generate. DO NOT EDIT.
//go:build mage

package main

// Rpm64 builds rpm package for amd64
func (p Package) Rpm64() error {
	var err error
	err = p.Rpm("amd64", "amd64")
	if err != nil {
		return err
	}

	return nil
}

// Rpm32 builds rpm package for 386
func (p Package) Rpm32() error {
	var err error
	err = p.Rpm("386", "i686")
	if err != nil {
		return err
	}

	return nil
}

// RpmArm64 builds rpm package for arm64 arm64
func (p Package) RpmArm64() error {
	var err error
	err = p.Rpm("arm64", "aarch64")
	if err != nil {
		return err
	}

	err = p.Rpm("arm64", "arm64")
	if err != nil {
		return err
	}

	return nil
}

// RpmArm32 builds rpm package for arm arm
func (p Package) RpmArm32() error {
	var err error
	err = p.Rpm("arm", "arm")
	if err != nil {
		return err
	}

	err = p.Rpm("arm", "armhf")
	if err != nil {
		return err
	}

	return nil
}

// RpmRiscv64 builds rpm package for riscv64
func (p Package) RpmRiscv64() error {
	var err error
	err = p.Rpm("riscv64", "riscv64")
	if err != nil {
		return err
	}

	return nil
}

// RpmIbm builds rpm package for s390x ppc64le
func (p Package) RpmIbm() error {
	var err error
	err = p.Rpm("s390x", "s390x")
	if err != nil {
		return err
	}

	err = p.Rpm("ppc64le", "ppc64el")
	if err != nil {
		return err
	}

	return nil
}
