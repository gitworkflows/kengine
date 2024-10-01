package integration

import (
	"testing"

	"github.com/khulnasoft/kengine/v2/kenginetest"
)

func TestDefaultSNI(t *testing.T) {
	// arrange
	tester := kenginetest.NewTester(t)
	tester.InitServer(`{
		"admin": {
			"listen": "localhost:2999"
		},
		"apps": {
			"http": {
				"http_port": 9080,
				"https_port": 9443,
				"grace_period": 1,
				"servers": {
					"srv0": {
						"listen": [
							":9443"
						],
						"routes": [
							{
								"handle": [
									{
										"handler": "subroute",
										"routes": [
											{
												"handle": [
													{
														"body": "hello from a.kengine.localhost",
														"handler": "static_response",
														"status_code": 200
													}
												],
												"match": [
													{
														"path": [
															"/version"
														]
													}
												]
											}
										]
									}
								],
								"match": [
									{
										"host": [
											"127.0.0.1"
										]
									}
								],
								"terminal": true
							}
						],
						"tls_connection_policies": [
							{
								"certificate_selection": {
									"any_tag": ["cert0"]
								},
								"match": {
									"sni": [
										"127.0.0.1"
									]
								}
							},
							{
								"default_sni": "*.kengine.localhost"
							}
						]
					}
				}
			},
			"tls": {
				"certificates": {
					"load_files": [
						{
							"certificate": "/kengine.localhost.crt",
							"key": "/kengine.localhost.key",
							"tags": [
								"cert0"
							]
						}
					]
				}
			},
			"pki": {
				"certificate_authorities" : {
					"local" : {
						"install_trust": false
					}
				}
			}
		}
	}
	`, "json")

	// act and assert
	// makes a request with no sni
	tester.AssertGetResponse("https://127.0.0.1:9443/version", 200, "hello from a.kengine.localhost")
}

func TestDefaultSNIWithNamedHostAndExplicitIP(t *testing.T) {
	// arrange
	tester := kenginetest.NewTester(t)
	tester.InitServer(` 
	{
		"admin": {
			"listen": "localhost:2999"
		},
		"apps": {
			"http": {
				"http_port": 9080,
				"https_port": 9443,
				"grace_period": 1,
				"servers": {
					"srv0": {
						"listen": [
							":9443"
						],
						"routes": [
							{
								"handle": [
									{
										"handler": "subroute",
										"routes": [
											{
												"handle": [
													{
														"body": "hello from a",
														"handler": "static_response",
														"status_code": 200
													}
												],
												"match": [
													{
														"path": [
															"/version"
														]
													}
												]
											}
										]
									}
								],
								"match": [
									{
										"host": [
											"a.kengine.localhost",
											"127.0.0.1"
										]
									}
								],
								"terminal": true
							}
						],
						"tls_connection_policies": [
							{
								"certificate_selection": {
									"any_tag": ["cert0"]
								},
								"default_sni": "a.kengine.localhost",
								"match": {
									"sni": [
										"a.kengine.localhost",
										"127.0.0.1",
										""
									]
								}
							},
							{
								"default_sni": "a.kengine.localhost"
							}
						]
					}
				}
			},
			"tls": {
				"certificates": {
					"load_files": [
						{
							"certificate": "/a.kengine.localhost.crt",
							"key": "/a.kengine.localhost.key",
							"tags": [
								"cert0"
							]
						}
					]
				}
			},
			"pki": {
				"certificate_authorities" : {
					"local" : {
						"install_trust": false
					}
				}
			}
		}
	}
	`, "json")

	// act and assert
	// makes a request with no sni
	tester.AssertGetResponse("https://127.0.0.1:9443/version", 200, "hello from a")
}

func TestDefaultSNIWithPortMappingOnly(t *testing.T) {
	// arrange
	tester := kenginetest.NewTester(t)
	tester.InitServer(` 
	{
		"admin": {
			"listen": "localhost:2999"
		},
		"apps": {
			"http": {
				"http_port": 9080,
				"https_port": 9443,
				"grace_period": 1,
				"servers": {
					"srv0": {
						"listen": [
							":9443"
						],
						"routes": [
							{
								"handle": [
									{
										"body": "hello from a.kengine.localhost",
										"handler": "static_response",
										"status_code": 200
									}
								],
								"match": [
									{
										"path": [
											"/version"
										]
									}
								]
							}
						],
						"tls_connection_policies": [
							{
								"certificate_selection": {
									"any_tag": ["cert0"]
								},
								"default_sni": "a.kengine.localhost"
							}
						]
					}
				}
			},
			"tls": {
				"certificates": {
					"load_files": [
						{
							"certificate": "/a.kengine.localhost.crt",
							"key": "/a.kengine.localhost.key",
							"tags": [
								"cert0"
							]
						}
					]
				}
			},
			"pki": {
				"certificate_authorities" : {
					"local" : {
						"install_trust": false
					}
				}
			}
		}
	}
	`, "json")

	// act and assert
	// makes a request with no sni
	tester.AssertGetResponse("https://127.0.0.1:9443/version", 200, "hello from a.kengine.localhost")
}

func TestHttpOnlyOnDomainWithSNI(t *testing.T) {
	kenginetest.AssertAdapt(t, `
	{
		skip_install_trust
		default_sni a.kengine.localhost
	}
	:80 {
		respond /version 200 {
			body "hello from localhost"
		}
	}
	`, "kenginefile", `{
	"apps": {
		"http": {
			"servers": {
				"srv0": {
					"listen": [
						":80"
					],
					"routes": [
						{
							"match": [
								{
									"path": [
										"/version"
									]
								}
							],
							"handle": [
								{
									"body": "hello from localhost",
									"handler": "static_response",
									"status_code": 200
								}
							]
						}
					]
				}
			}
		},
		"pki": {
			"certificate_authorities": {
				"local": {
					"install_trust": false
				}
			}
		}
	}
}`)
}
