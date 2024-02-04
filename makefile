
install_k3s:
	curl -sfL https://get.k3s.io | sh -s - --disable=traefik && \
	sudo cp /etc/rancher/k3s/k3s.yaml /home/$$USER/.kube/config
	sudo chown -R $$USER /home/$$USER/.kube/config
