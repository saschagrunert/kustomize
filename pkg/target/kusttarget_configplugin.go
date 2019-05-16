/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package target

import (
	"github.com/pkg/errors"
	"sigs.k8s.io/kustomize/pkg/image"
	"sigs.k8s.io/kustomize/pkg/plugins"
	"sigs.k8s.io/kustomize/pkg/transformers"
	"sigs.k8s.io/kustomize/pkg/transformers/config"
	"sigs.k8s.io/kustomize/pkg/types"
	"sigs.k8s.io/kustomize/plugin/builtingen"
	"sigs.k8s.io/yaml"
)

// Functions dedicated to configuring the builtin
// transformer and generator plugins using config data
// read from a kustomization file.
//
// Non-builtin plugins will get their configuration
// from their own dedicated structs and yaml files.
//
// There are some loops in the functions below because
// the kustomization file would, say, allow one to
// request multiple secrets be made, or run multiple
// image tag transforms, so we need to run the plugins
// N times (plugins are easier to write, configure and
// test if they do just one thing).
//
// TODO: Push code down into the plugins, as the first pass
//     at this writes plugins as thin layers over calls
//     into existing packages.  The builtin plugins should
//     be viewed as examples, and the packages they access
//     directory should be public, while everything else
//     should go into internal.

type generatorConfigurator func() ([]transformers.Generator, error)
type transformerConfigurator func(
	tConfig *config.TransformerConfig) ([]transformers.Transformer, error)

func (kt *KustTarget) configureBuiltinGenerators() (
	[]transformers.Generator, error) {
	configurators := []generatorConfigurator{
		kt.configureBuiltinConfigMapGenerator,
		kt.configureBuiltinSecretGenerator,
	}
	var result []transformers.Generator
	for _, f := range configurators {
		r, err := f()
		if err != nil {
			return nil, err
		}
		result = append(result, r...)
	}
	return result, nil
}

func (kt *KustTarget) configureBuiltinTransformers(
	tConfig *config.TransformerConfig) (
	[]transformers.Transformer, error) {
	// TODO: Convert remaining legacy transformers to plugins
	//     (patch SMP/JSON, name prefix/suffix, labels/annos).
	//     with tests.
	configurators := []transformerConfigurator{
		kt.configureBuiltinImageTagTransformer,
	}
	var result []transformers.Transformer
	for _, f := range configurators {
		r, err := f(tConfig)
		if err != nil {
			return nil, err
		}
		result = append(result, r...)
	}
	return result, nil
}

func (kt *KustTarget) configureBuiltinSecretGenerator() (
	result []transformers.Generator, err error) {
	var c struct {
		types.GeneratorOptions
		types.SecretArgs
	}
	if kt.kustomization.GeneratorOptions != nil {
		c.GeneratorOptions = *kt.kustomization.GeneratorOptions
	}
	for _, args := range kt.kustomization.SecretGenerator {
		c.SecretArgs = args
		p := builtingen.NewSecretGeneratorPlugin()
		err = kt.configureBuiltinPlugin(p, c, "secret")
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return
}

func (kt *KustTarget) configureBuiltinConfigMapGenerator() (
	result []transformers.Generator, err error) {
	var c struct {
		types.GeneratorOptions
		types.ConfigMapArgs
	}
	if kt.kustomization.GeneratorOptions != nil {
		c.GeneratorOptions = *kt.kustomization.GeneratorOptions
	}
	for _, args := range kt.kustomization.ConfigMapGenerator {
		c.ConfigMapArgs = args
		p := builtingen.NewConfigMapGeneratorPlugin()
		err = kt.configureBuiltinPlugin(p, c, "configmap")
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return
}

func (kt *KustTarget) configureBuiltinImageTagTransformer(
	tConfig *config.TransformerConfig) (
	result []transformers.Transformer, err error) {
	var c struct {
		ImageTag   image.Image
		FieldSpecs []config.FieldSpec
	}
	for _, args := range kt.kustomization.Images {
		c.ImageTag = args
		c.FieldSpecs = tConfig.Images
		p := builtingen.NewImageTagTransformerPlugin()
		err = kt.configureBuiltinPlugin(p, c, "imageTag")
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return
}

func (kt *KustTarget) configureBuiltinPlugin(
	p plugins.Configurable, c interface{}, id string) error {
	y, err := yaml.Marshal(c)
	if err != nil {
		return errors.Wrapf(err, "builtin %s marshal", id)
	}
	err = p.Config(kt.ldr, kt.rFactory, y)
	if err != nil {
		return errors.Wrapf(err, "builtin %s config: %v", id, y)
	}
	return nil
}