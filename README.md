# General idea
This is a command line interface for https://github.com/oscp/openshift-monitoring. It is used in our external monitoring system. It calls checks on the local server and reports them back to the monitoring tool. For more details about the check, visit https://github.com/oscp/openshift-monitoring.

## Usage & Configuration
Start from the configuration template 'config.template.yml'. Place your own config file at the same location as the binary then run:

```bash
openshift-monitoring-cli 
```

## Checks
**TYPE**|**MAJOR / MINOR** |**CHECK** 
|---------|---------------|---------------------------------------------------------| 
| NODE    | MINOR         | Checks if the dockerpool is > 80%                       
|         |               | Checks ntpd synchronization status                       
|         |               | Checks if http access via service is ok        
| NODE    | MAJOR         | Checks if the dockerpool is > 90%                        
|         |               | Check if dns is ok via kubernetes & dnsmasq              
| MASTER  | MINOR         | Checks ntpd synchronization status                       
|         |               | Checks if external system is reachable                   
|         |               | Checks if hawcular is healthy                            
|         |               | Checks if ha-proxy has a high restart count              
|         |               | Checks if all projects have limits & quotas              
|         |               | Checks if logging pods are healthy                      
|         |               | Checks if http access via service is ok       
| MASTER  | MAJOR         | Checks if output of 'oc get nodes' is fine               
|         |               | Checks if etcd cluster is healthy                        
|         |               | Checks if docker registry is healthy                     
|         |               | Checks if all routers are healthy                        
|         |               | Checks if local master api is healthy                    
|         |               | Check if dns is ok via kubernetes & dnsmasq             
| STORAGE | MINOR         | Checks if open-files count is higher than 200'000 files  
|         |               | Checks every lvs-pool size. Is the value above 80%?      
|         |               | Checks every VG has at least 10% free storage            
|         |               | Checks if every specified mount path has at least 15% free storage            
| STORAGE | MAJOR         | Checks if output of gstatus is 'healthy'                 
|         |               | Checks every lvs-pool size. Is the value above 90%?      
|         |               | Checks every VG has at least 5% free storage             
|         |               | Checks if every specified mount path has at least 10% free storage            