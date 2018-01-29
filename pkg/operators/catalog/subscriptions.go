package catalog

import (
	"errors"
	"fmt"

	ipv1alpha1 "github.com/coreos-inc/alm/pkg/apis/installplan/v1alpha1"
	"github.com/coreos-inc/alm/pkg/apis/subscription/v1alpha1"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrNilSubscription = errors.New("invalid Subscription object: <nil>")
)

func (o *Operator) syncSubscription(sub *v1alpha1.Subscription) error {
	if sub == nil || sub.Spec == nil {
		return ErrNilSubscription
	}
	// only sync if catalog has been updated since last sync time
	if o.sourcesLastUpdate.Before(&sub.Status.LastUpdated) {
		log.Infof("skipping sync: no new updates to catalog since last sync at %s",
			sub.Status.LastUpdated.String())
		return nil
	}
	o.sourcesLock.Lock()
	defer o.sourcesLock.Unlock()
	catalog, ok := o.sources[sub.Spec.CatalogSource]
	if !ok {
		return fmt.Errorf("unknown catalog source %s", sub.Spec.CatalogSource)
	}
	// find latest CSV if no CSVs are installed already
	if sub.Status.CurrentCSV == "" {
		if sub.Spec.StartingCSV != "" {
			sub.Status.CurrentCSV = sub.Spec.StartingCSV
		} else {
			csv, err := catalog.FindCSVForPackageNameUnderChannel(sub.Spec.Package, sub.Spec.Channel)
			if err != nil {
				return fmt.Errorf("failed to find CSV for package %s in channel %s: %v",
					sub.Spec.Package, sub.Spec.Channel, err)
			}
			if csv == nil {
				return fmt.Errorf("failed to find CSV for package %s in channel %s: nil CSV",
					sub.Spec.Package, sub.Spec.Channel)
			}
			sub.Status.CurrentCSV = csv.GetName()
		}
		_, err := o.subscriptionClient.UpdateSubscription(sub)
		return err
	}
	if sub.Status.CurrentCSV == "" {

	}
	// check that desired CSV has been installed
	csv, err := o.csvClient.GetCSVByName(sub.GetNamespace(), sub.Status.CurrentCSV)
	if err != nil || csv == nil {
		log.Infof("error fetching CSV %s via k8s api: %v", sub.Status.CurrentCSV, err)
		if sub.Status.Install != nil && sub.Status.Install.Name != "" {
			ip, err := o.ipClient.GetInstallPlanByName(sub.GetNamespace(), sub.Status.Install.Name)
			if err != nil {
				log.Errorf("get installplan %s error: %v", sub.Status.Install.Name, err)
			}
			if err == nil && ip != nil {
				log.Infof("installplan for %s already exists", sub.Status.CurrentCSV)
				return nil
			}
			log.Infof("installplan %s not found: creating new plan", sub.Status.Install.Name)
			sub.Status.Install = nil
		}
		// install CSV if doesn't exist
		ip := &ipv1alpha1.InstallPlan{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: ipv1alpha1.InstallPlanSpec{
				ClusterServiceVersionNames: []string{sub.Status.CurrentCSV},
				Approval:                   ipv1alpha1.ApprovalAutomatic,
			},
		}
		owner := []metav1.OwnerReference{
			{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       v1alpha1.SubscriptionKind,
				Name:       sub.GetName(),
				UID:        sub.GetUID(),
			},
		}
		ip.SetOwnerReferences(owner)
		ip.SetGenerateName(fmt.Sprintf("install-%s", sub.Status.CurrentCSV))
		ip.SetNamespace(sub.GetNamespace())
		res, err := o.ipClient.CreateInstallPlan(ip)
		if err != nil {
			return fmt.Errorf("failed to ensure current CSV %s installed: %v", sub.Status.CurrentCSV, err)
		}
		if res == nil {
			return errors.New("unexpected installplan returned by k8s api on create: <nil>")
		}
		sub.Status.Install = &v1alpha1.InstallPlanReference{
			UID:        res.GetUID(),
			Name:       res.GetName(),
			APIVersion: res.TypeMeta.APIVersion,
			Kind:       res.TypeMeta.Kind,
		}
		_, err = o.subscriptionClient.UpdateSubscription(sub)
		return err
	}
	// poll catalog for an update
	repl, err := catalog.FindReplacementCSVForPackageNameUnderChannel(
		sub.Spec.Package, sub.Spec.Channel, sub.Status.CurrentCSV)
	if err != nil {
		return fmt.Errorf("failed to lookup replacement CSV for %s: %v", sub.Status.CurrentCSV, err)
	}
	if repl == nil {
		return fmt.Errorf("nil replacement CSV for %s returned from catalog", sub.Status.CurrentCSV)
	}
	// update subscription with new latest
	sub.Status.CurrentCSV = repl.GetName()
	sub.Status.Install = nil
	_, err = o.subscriptionClient.UpdateSubscription(sub)
	return err
}
