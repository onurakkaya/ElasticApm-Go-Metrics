## Elastic Apm - Example Go Project
ElasticApm Go Agent integration tutorial.

Application UI (Referenced Web Application - https://freshman.tech/web-development-with-go/)
![ui](#url)
Elastic Apm - Kibana 
![apm](#url)
## Prerequisites
You need to have [Go](https://golang.org/dl/) installed on your computer. The version used to test the code in the tutorial is **1.15.3**.
## Get started
You can run project with 

> fresh

 command from terminal.

---
To configure ElasticApm Agent you can modify 

> .env

 file in root directory.

### Configuration Management
These keys are required to send data to apm server.
- ELASTIC_APM_SERVER_URL= http://127.0.0.1:8200 
- ELASTIC_APM_SECRET_TOKEN="A base64 string"
- ELASTIC_APM_SERVICE_NAME=Apm_Hello_Go(Display Name)

Normally apm agent reads the apm configuration from Environment Variables before the application start. You can set the variables above and start to send data simply. 

But some situations, you can be need to start agent after the boot of the app or reconfigure the application parameters on running application.

To reload Apm agent configuration. You can call the transport.InitDefault() method.

    import ("go.elastic.co/apm/transport") // Requires
    
    // Reload Environment variables to Elastic Apm.
    if _, err := transport.InitDefault(); err !=  nil {
    log.Fatal(err)
    }

After do this you need to replace Default Apm Tracer reference by new one to apply to the changes.

    serviceName := os.Getenv("ELASTIC_APM_SERVICE_NAME")
    serviceVersion := os.Getenv("ELASTIC_APM_SERVICE_VERSION")
    tracer, err := apm.NewTracer(serviceName, serviceVersion)
    apm.DefaultTracer = tracer

At least you need to update the HttpListener parameters with the code bellow.

    http.ListenAndServe(":"+port, apmhttp.Wrap(mux, apmhttp.WithTracer(tracer)))

### Custom Data Collection 
Sample of custom Sub Operations(spans)  on current Transaction. You can  gather the current transaction's reference by HttpContext object.

    func(w http.ResponseWriter, r *http.Request){ 
	...
    ctx := r.Context()
    apm.DefaultTracer.CaptureHTTPRequestBody(r)
   
    parentSpan, ctx := apm.StartSpan(ctx, searchQuery, "NewsAPI")
    parentSpan.Action = "FetchAPI"
    ...
    parentSpan.End() // This command ends up the started span.
    }
  
You can start spans of span for your subsequent operations. 

    childSpan := apm.DefaultTracer.StartSpan(resultString, "NewsApi",parentSpan.TraceContext().Span, spanOptions)
    childSpan.SetStacktrace(0)
    childSpan.End()


