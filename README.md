# scloud-register
go 语言 链接 spring-cloud 注册中心 


## Usage

Import

    import "github.com/haozailai8/scloud-register"
    
In your code, call the Register method:

    eeureka.Register("myMicroservice", "8080", "443")
    
or
    
    eeureka.RegisterAt("http://192.168.123.123:8761","myMicroservice", "8080", "443")
