// @generated
/// Generated client implementations.
pub mod query_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct QueryServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl QueryServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> QueryServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> QueryServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            QueryServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn run_query(
            &mut self,
            request: impl tonic::IntoRequest<super::RunQueryRequest>,
        ) -> std::result::Result<
            tonic::Response<super::RunQueryResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.QueryService/RunQuery",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.QueryService", "RunQuery"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn run_queries(
            &mut self,
            request: impl tonic::IntoRequest<super::RunQueriesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::RunQueriesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.QueryService/RunQueries",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.QueryService", "RunQueries"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_relations(
            &mut self,
            request: impl tonic::IntoRequest<super::ListRelationsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListRelationsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.QueryService/ListRelations",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.QueryService", "ListRelations"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_query_sets(
            &mut self,
            request: impl tonic::IntoRequest<super::ListQuerySetsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListQuerySetsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.QueryService/ListQuerySets",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.QueryService", "ListQuerySets"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn parse_query_set(
            &mut self,
            request: impl tonic::IntoRequest<super::ParseQuerySetRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ParseQuerySetResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.QueryService/ParseQuerySet",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.QueryService", "ParseQuerySet"));
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod check_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct CheckServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl CheckServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> CheckServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> CheckServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            CheckServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn list_rules(
            &mut self,
            request: impl tonic::IntoRequest<super::ListRulesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListRulesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/ListRules",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.CheckService", "ListRules"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn check_design(
            &mut self,
            request: impl tonic::IntoRequest<super::CheckDesignRequest>,
        ) -> std::result::Result<
            tonic::Response<super::CheckDesignResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/CheckDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.CheckService", "CheckDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_expectations(
            &mut self,
            request: impl tonic::IntoRequest<super::GetExpectationsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetExpectationsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/GetExpectations",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.CheckService", "GetExpectations"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_check_report(
            &mut self,
            request: impl tonic::IntoRequest<super::GetCheckReportRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetCheckReportResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/GetCheckReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.CheckService", "GetCheckReport"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn render_check_report(
            &mut self,
            request: impl tonic::IntoRequest<super::RenderCheckReportRequest>,
        ) -> std::result::Result<
            tonic::Response<super::RenderCheckReportResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/RenderCheckReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.CheckService", "RenderCheckReport"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_interface_coverage(
            &mut self,
            request: impl tonic::IntoRequest<super::GetInterfaceCoverageRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetInterfaceCoverageResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/GetInterfaceCoverage",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new(
                        "agni.v1.webapi.CheckService",
                        "GetInterfaceCoverage",
                    ),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_component_params(
            &mut self,
            request: impl tonic::IntoRequest<super::GetComponentParamsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetComponentParamsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/GetComponentParams",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.CheckService", "GetComponentParams"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_naming_convention(
            &mut self,
            request: impl tonic::IntoRequest<super::GetNamingConventionRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetNamingConventionResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.CheckService/GetNamingConvention",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.CheckService", "GetNamingConvention"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod design_view_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct DesignViewServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl DesignViewServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> DesignViewServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> DesignViewServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            DesignViewServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn get_summary(
            &mut self,
            request: impl tonic::IntoRequest<super::GetSummaryRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetSummaryResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/GetSummary",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "GetSummary"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_sheet(
            &mut self,
            request: impl tonic::IntoRequest<super::GetSheetRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetSheetResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/GetSheet",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignViewService", "GetSheet"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn highlight_sheet(
            &mut self,
            request: impl tonic::IntoRequest<super::HighlightSheetRequest>,
        ) -> std::result::Result<
            tonic::Response<super::HighlightSheetResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/HighlightSheet",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "HighlightSheet"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_layout_report(
            &mut self,
            request: impl tonic::IntoRequest<super::GetLayoutReportRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetLayoutReportResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/GetLayoutReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new(
                        "agni.v1.webapi.DesignViewService",
                        "GetLayoutReport",
                    ),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_board_geometry(
            &mut self,
            request: impl tonic::IntoRequest<super::GetBoardGeometryRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetBoardGeometryResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/GetBoardGeometry",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new(
                        "agni.v1.webapi.DesignViewService",
                        "GetBoardGeometry",
                    ),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_board_meshes(
            &mut self,
            request: impl tonic::IntoRequest<super::GetBoardMeshesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetBoardMeshesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/GetBoardMeshes",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "GetBoardMeshes"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn locate_subjects(
            &mut self,
            request: impl tonic::IntoRequest<super::LocateSubjectsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::LocateSubjectsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/LocateSubjects",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "LocateSubjects"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn trace_design(
            &mut self,
            request: impl tonic::IntoRequest<super::TraceDesignRequest>,
        ) -> std::result::Result<
            tonic::Response<super::TraceDesignResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/TraceDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "TraceDesign"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn reachable_pins(
            &mut self,
            request: impl tonic::IntoRequest<super::ReachablePinsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ReachablePinsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/ReachablePins",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "ReachablePins"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn walk_design(
            &mut self,
            request: impl tonic::IntoRequest<super::WalkDesignRequest>,
        ) -> std::result::Result<
            tonic::Response<super::WalkDesignResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/WalkDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "WalkDesign"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_walk_kinds(
            &mut self,
            request: impl tonic::IntoRequest<super::ListWalkKindsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListWalkKindsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignViewService/ListWalkKinds",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignViewService", "ListWalkKinds"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod project_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct ProjectServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl ProjectServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> ProjectServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> ProjectServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            ProjectServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn get_project(
            &mut self,
            request: impl tonic::IntoRequest<super::GetProjectRequest>,
        ) -> std::result::Result<tonic::Response<super::Project>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/GetProject",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.ProjectService", "GetProject"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_projects(
            &mut self,
            request: impl tonic::IntoRequest<super::ListProjectsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListProjectsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/ListProjects",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ProjectService", "ListProjects"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn create_project(
            &mut self,
            request: impl tonic::IntoRequest<super::CreateProjectRequest>,
        ) -> std::result::Result<tonic::Response<super::Project>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/CreateProject",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ProjectService", "CreateProject"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn update_project(
            &mut self,
            request: impl tonic::IntoRequest<super::UpdateProjectRequest>,
        ) -> std::result::Result<tonic::Response<super::Project>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/UpdateProject",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ProjectService", "UpdateProject"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_project(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteProjectRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/DeleteProject",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ProjectService", "DeleteProject"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn resolve_design(
            &mut self,
            request: impl tonic::IntoRequest<super::ResolveDesignRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ResolveDesignResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ProjectService/ResolveDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ProjectService", "ResolveDesign"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod snapshot_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct SnapshotServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl SnapshotServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> SnapshotServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> SnapshotServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            SnapshotServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn ingest(
            &mut self,
            request: impl tonic::IntoRequest<super::IngestRequest>,
        ) -> std::result::Result<tonic::Response<super::Snapshot>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/Ingest",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.SnapshotService", "Ingest"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_snapshot(
            &mut self,
            request: impl tonic::IntoRequest<super::GetSnapshotRequest>,
        ) -> std::result::Result<tonic::Response<super::Snapshot>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/GetSnapshot",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.SnapshotService", "GetSnapshot"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_snapshots(
            &mut self,
            request: impl tonic::IntoRequest<super::ListSnapshotsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListSnapshotsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/ListSnapshots",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.SnapshotService", "ListSnapshots"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn update_snapshot(
            &mut self,
            request: impl tonic::IntoRequest<super::UpdateSnapshotRequest>,
        ) -> std::result::Result<tonic::Response<super::Snapshot>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/UpdateSnapshot",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.SnapshotService", "UpdateSnapshot"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_snapshot(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteSnapshotRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/DeleteSnapshot",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.SnapshotService", "DeleteSnapshot"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn upload_blobs(
            &mut self,
            request: impl tonic::IntoRequest<super::UploadBlobsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::UploadBlobsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.SnapshotService/UploadBlobs",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.SnapshotService", "UploadBlobs"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod design_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct DesignServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl DesignServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> DesignServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> DesignServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            DesignServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn create_design(
            &mut self,
            request: impl tonic::IntoRequest<super::CreateDesignRequest>,
        ) -> std::result::Result<tonic::Response<super::Design>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/CreateDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "CreateDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_design(
            &mut self,
            request: impl tonic::IntoRequest<super::GetDesignRequest>,
        ) -> std::result::Result<tonic::Response<super::Design>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/GetDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "GetDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_designs(
            &mut self,
            request: impl tonic::IntoRequest<super::ListDesignsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListDesignsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/ListDesigns",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "ListDesigns"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn update_design(
            &mut self,
            request: impl tonic::IntoRequest<super::UpdateDesignRequest>,
        ) -> std::result::Result<tonic::Response<super::Design>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/UpdateDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "UpdateDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_design(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteDesignRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/DeleteDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "DeleteDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_artifact(
            &mut self,
            request: impl tonic::IntoRequest<super::GetArtifactRequest>,
        ) -> std::result::Result<tonic::Response<super::Artifact>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/GetArtifact",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "GetArtifact"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_artifacts(
            &mut self,
            request: impl tonic::IntoRequest<super::ListArtifactsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListArtifactsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/ListArtifacts",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignService", "ListArtifacts"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn update_artifact(
            &mut self,
            request: impl tonic::IntoRequest<super::UpdateArtifactRequest>,
        ) -> std::result::Result<tonic::Response<super::Artifact>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/UpdateArtifact",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignService", "UpdateArtifact"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn batch_update_artifacts(
            &mut self,
            request: impl tonic::IntoRequest<super::BatchUpdateArtifactsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::BatchUpdateArtifactsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/BatchUpdateArtifacts",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new(
                        "agni.v1.webapi.DesignService",
                        "BatchUpdateArtifacts",
                    ),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn import_design_files(
            &mut self,
            request: impl tonic::IntoRequest<super::ImportDesignFilesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ImportDesignFilesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/ImportDesignFiles",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignService", "ImportDesignFiles"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_artifact(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteArtifactRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/DeleteArtifact",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignService", "DeleteArtifact"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn link_artifact(
            &mut self,
            request: impl tonic::IntoRequest<super::LinkArtifactRequest>,
        ) -> std::result::Result<tonic::Response<super::Artifact>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/LinkArtifact",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "LinkArtifact"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn unlink_artifact(
            &mut self,
            request: impl tonic::IntoRequest<super::UnlinkArtifactRequest>,
        ) -> std::result::Result<tonic::Response<super::Artifact>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/UnlinkArtifact",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.DesignService", "UnlinkArtifact"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn sync_design(
            &mut self,
            request: impl tonic::IntoRequest<super::SyncDesignRequest>,
        ) -> std::result::Result<
            tonic::Response<super::SyncDesignResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/SyncDesign",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "SyncDesign"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn create_report(
            &mut self,
            request: impl tonic::IntoRequest<super::CreateReportRequest>,
        ) -> std::result::Result<tonic::Response<super::Report>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/CreateReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "CreateReport"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_report(
            &mut self,
            request: impl tonic::IntoRequest<super::GetReportRequest>,
        ) -> std::result::Result<tonic::Response<super::Report>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/GetReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "GetReport"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_reports(
            &mut self,
            request: impl tonic::IntoRequest<super::ListReportsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListReportsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/ListReports",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "ListReports"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_report(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteReportRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DesignService/DeleteReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DesignService", "DeleteReport"));
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod diff_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct DiffServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl DiffServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> DiffServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> DiffServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            DiffServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn diff_designs(
            &mut self,
            request: impl tonic::IntoRequest<super::DiffDesignsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::DiffDesignsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.DiffService/DiffDesigns",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.DiffService", "DiffDesigns"));
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod review_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct ReviewServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl ReviewServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> ReviewServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> ReviewServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            ReviewServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn create_review(
            &mut self,
            request: impl tonic::IntoRequest<super::CreateReviewRequest>,
        ) -> std::result::Result<tonic::Response<super::Review>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/CreateReview",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.ReviewService", "CreateReview"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_review(
            &mut self,
            request: impl tonic::IntoRequest<super::GetReviewRequest>,
        ) -> std::result::Result<tonic::Response<super::Review>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/GetReview",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.ReviewService", "GetReview"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_reviews(
            &mut self,
            request: impl tonic::IntoRequest<super::ListReviewsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListReviewsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/ListReviews",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.ReviewService", "ListReviews"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_review(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteReviewRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/DeleteReview",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.ReviewService", "DeleteReview"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_review_manifest(
            &mut self,
            request: impl tonic::IntoRequest<super::GetReviewManifestRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetReviewManifestResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/GetReviewManifest",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ReviewService", "GetReviewManifest"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_checklists(
            &mut self,
            request: impl tonic::IntoRequest<super::ListChecklistsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListChecklistsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/ListChecklists",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ReviewService", "ListChecklists"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn render_review_report(
            &mut self,
            request: impl tonic::IntoRequest<super::RenderReviewReportRequest>,
        ) -> std::result::Result<
            tonic::Response<super::RenderReviewReportResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ReviewService/RenderReviewReport",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ReviewService", "RenderReviewReport"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod table_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct TableServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl TableServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> TableServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> TableServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            TableServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn tabulate(
            &mut self,
            request: impl tonic::IntoRequest<super::TabulateRequest>,
        ) -> std::result::Result<
            tonic::Response<super::TabulateResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.TableService/Tabulate",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.TableService", "Tabulate"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn encode(
            &mut self,
            request: impl tonic::IntoRequest<super::EncodeTablesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::EncodeTablesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.TableService/Encode",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.TableService", "Encode"));
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod lexicon_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct LexiconServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl LexiconServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> LexiconServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> LexiconServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            LexiconServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn get_lexicon(
            &mut self,
            request: impl tonic::IntoRequest<super::GetLexiconRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetLexiconResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.LexiconService/GetLexicon",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.LexiconService", "GetLexicon"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn explain_net(
            &mut self,
            request: impl tonic::IntoRequest<super::ExplainNetRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ExplainNetResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.LexiconService/ExplainNet",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.LexiconService", "ExplainNet"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn compare_lexicons(
            &mut self,
            request: impl tonic::IntoRequest<super::CompareLexiconsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::CompareLexiconsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.LexiconService/CompareLexicons",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.LexiconService", "CompareLexicons"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn render_convention(
            &mut self,
            request: impl tonic::IntoRequest<super::RenderConventionRequest>,
        ) -> std::result::Result<
            tonic::Response<super::RenderConventionResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.LexiconService/RenderConvention",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.LexiconService", "RenderConvention"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod pack_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct PackServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl PackServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> PackServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> PackServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            PackServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn create_pack(
            &mut self,
            request: impl tonic::IntoRequest<super::CreatePackRequest>,
        ) -> std::result::Result<tonic::Response<super::Pack>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/CreatePack",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.PackService", "CreatePack"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_pack(
            &mut self,
            request: impl tonic::IntoRequest<super::GetPackRequest>,
        ) -> std::result::Result<tonic::Response<super::Pack>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/GetPack",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.PackService", "GetPack"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_packs(
            &mut self,
            request: impl tonic::IntoRequest<super::ListPacksRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListPacksResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/ListPacks",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.PackService", "ListPacks"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn update_pack(
            &mut self,
            request: impl tonic::IntoRequest<super::UpdatePackRequest>,
        ) -> std::result::Result<tonic::Response<super::Pack>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/UpdatePack",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.PackService", "UpdatePack"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_pack(
            &mut self,
            request: impl tonic::IntoRequest<super::DeletePackRequest>,
        ) -> std::result::Result<tonic::Response<::pbjson_types::Empty>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/DeletePack",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.PackService", "DeletePack"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn import_pack_files(
            &mut self,
            request: impl tonic::IntoRequest<super::ImportPackFilesRequest>,
        ) -> std::result::Result<tonic::Response<super::Pack>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.PackService/ImportPackFiles",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.PackService", "ImportPackFiles"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod server_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct ServerServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl ServerServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> ServerServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> ServerServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            ServerServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn get_server_info(
            &mut self,
            request: impl tonic::IntoRequest<super::GetServerInfoRequest>,
        ) -> std::result::Result<tonic::Response<super::ServerInfo>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ServerService/GetServerInfo",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ServerService", "GetServerInfo"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod viewer_room_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct ViewerRoomServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl ViewerRoomServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> ViewerRoomServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> ViewerRoomServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            ViewerRoomServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn join_viewer_room(
            &mut self,
            request: impl tonic::IntoRequest<super::JoinViewerRoomRequest>,
        ) -> std::result::Result<
            tonic::Response<super::JoinViewerRoomResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.ViewerRoomService/JoinViewerRoom",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.ViewerRoomService", "JoinViewerRoom"),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
/// Generated client implementations.
pub mod workspace_service_client {
    #![allow(
        unused_variables,
        dead_code,
        missing_docs,
        clippy::wildcard_imports,
        clippy::let_unit_value,
    )]
    use tonic::codegen::*;
    use tonic::codegen::http::Uri;
    #[derive(Debug, Clone)]
    pub struct WorkspaceServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl WorkspaceServiceClient<tonic::transport::Channel> {
        /// Attempt to create a new client by connecting to a given endpoint.
        pub async fn connect<D>(dst: D) -> Result<Self, tonic::transport::Error>
        where
            D: TryInto<tonic::transport::Endpoint>,
            D::Error: Into<StdError>,
        {
            let conn = tonic::transport::Endpoint::new(dst)?.connect().await?;
            Ok(Self::new(conn))
        }
    }
    impl<T> WorkspaceServiceClient<T>
    where
        T: tonic::client::GrpcService<tonic::body::Body>,
        T::Error: Into<StdError>,
        T::ResponseBody: Body<Data = Bytes> + std::marker::Send + 'static,
        <T::ResponseBody as Body>::Error: Into<StdError> + std::marker::Send,
    {
        pub fn new(inner: T) -> Self {
            let inner = tonic::client::Grpc::new(inner);
            Self { inner }
        }
        pub fn with_origin(inner: T, origin: Uri) -> Self {
            let inner = tonic::client::Grpc::with_origin(inner, origin);
            Self { inner }
        }
        pub fn with_interceptor<F>(
            inner: T,
            interceptor: F,
        ) -> WorkspaceServiceClient<InterceptedService<T, F>>
        where
            F: tonic::service::Interceptor,
            T::ResponseBody: Default,
            T: tonic::codegen::Service<
                http::Request<tonic::body::Body>,
                Response = http::Response<
                    <T as tonic::client::GrpcService<tonic::body::Body>>::ResponseBody,
                >,
            >,
            <T as tonic::codegen::Service<
                http::Request<tonic::body::Body>,
            >>::Error: Into<StdError> + std::marker::Send + std::marker::Sync,
        {
            WorkspaceServiceClient::new(InterceptedService::new(inner, interceptor))
        }
        /// Compress requests with the given encoding.
        ///
        /// This requires the server to support it otherwise it might respond with an
        /// error.
        #[must_use]
        pub fn send_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.send_compressed(encoding);
            self
        }
        /// Enable decompressing responses.
        #[must_use]
        pub fn accept_compressed(mut self, encoding: CompressionEncoding) -> Self {
            self.inner = self.inner.accept_compressed(encoding);
            self
        }
        /// Limits the maximum size of a decoded message.
        ///
        /// Default: `4MB`
        #[must_use]
        pub fn max_decoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_decoding_message_size(limit);
            self
        }
        /// Limits the maximum size of an encoded message.
        ///
        /// Default: `usize::MAX`
        #[must_use]
        pub fn max_encoding_message_size(mut self, limit: usize) -> Self {
            self.inner = self.inner.max_encoding_message_size(limit);
            self
        }
        pub async fn list_mounts(
            &mut self,
            request: impl tonic::IntoRequest<super::ListMountsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListMountsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/ListMounts",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.WorkspaceService", "ListMounts"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn get_mount(
            &mut self,
            request: impl tonic::IntoRequest<super::GetMountRequest>,
        ) -> std::result::Result<tonic::Response<super::Mount>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/GetMount",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.WorkspaceService", "GetMount"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn create_mount(
            &mut self,
            request: impl tonic::IntoRequest<super::CreateMountRequest>,
        ) -> std::result::Result<tonic::Response<super::Mount>, tonic::Status> {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/CreateMount",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.WorkspaceService", "CreateMount"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn delete_mount(
            &mut self,
            request: impl tonic::IntoRequest<super::DeleteMountRequest>,
        ) -> std::result::Result<
            tonic::Response<super::DeleteMountResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/DeleteMount",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.WorkspaceService", "DeleteMount"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_dir(
            &mut self,
            request: impl tonic::IntoRequest<super::ListDirRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListDirResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/ListDir",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.webapi.WorkspaceService", "ListDir"));
            self.inner.unary(req, path, codec).await
        }
        pub async fn list_design_files(
            &mut self,
            request: impl tonic::IntoRequest<super::ListDesignFilesRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ListDesignFilesResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/ListDesignFiles",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.WorkspaceService", "ListDesignFiles"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn propose_designs(
            &mut self,
            request: impl tonic::IntoRequest<super::ProposeDesignsRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ProposeDesignsResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/ProposeDesigns",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.webapi.WorkspaceService", "ProposeDesigns"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn declare_companion(
            &mut self,
            request: impl tonic::IntoRequest<super::DeclareCompanionRequest>,
        ) -> std::result::Result<
            tonic::Response<super::DeclareCompanionResponse>,
            tonic::Status,
        > {
            self.inner
                .ready()
                .await
                .map_err(|e| {
                    tonic::Status::unknown(
                        format!("Service was not ready: {}", e.into()),
                    )
                })?;
            let codec = tonic_prost::ProstCodec::default();
            let path = http::uri::PathAndQuery::from_static(
                "/agni.v1.webapi.WorkspaceService/DeclareCompanion",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new(
                        "agni.v1.webapi.WorkspaceService",
                        "DeclareCompanion",
                    ),
                );
            self.inner.unary(req, path, codec).await
        }
    }
}
