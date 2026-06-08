kubectl apply -f ./install.yaml -n argocd 
kubectl port-forward -n argocd svc/argocd-server 8080:443


Login 
Admin 
Get password with: 
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 --decode
