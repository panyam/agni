// @generated
/// Generated client implementations.
pub mod viewer_page_service_client {
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
    pub struct ViewerPageServiceClient<T> {
        inner: tonic::client::Grpc<T>,
    }
    impl ViewerPageServiceClient<tonic::transport::Channel> {
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
    impl<T> ViewerPageServiceClient<T>
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
        ) -> ViewerPageServiceClient<InterceptedService<T, F>>
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
            ViewerPageServiceClient::new(InterceptedService::new(inner, interceptor))
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
        pub async fn get_location(
            &mut self,
            request: impl tonic::IntoRequest<super::GetLocationRequest>,
        ) -> std::result::Result<
            tonic::Response<super::GetLocationResponse>,
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
                "/agni.v1.viewer.ViewerPageService/GetLocation",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "GetLocation"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn apply_location(
            &mut self,
            request: impl tonic::IntoRequest<super::ApplyLocationRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ApplyLocationResponse>,
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
                "/agni.v1.viewer.ViewerPageService/ApplyLocation",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "ApplyLocation"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn locate_verdict(
            &mut self,
            request: impl tonic::IntoRequest<super::LocateVerdictRequest>,
        ) -> std::result::Result<
            tonic::Response<super::LocateVerdictResponse>,
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
                "/agni.v1.viewer.ViewerPageService/LocateVerdict",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "LocateVerdict"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn select_finding(
            &mut self,
            request: impl tonic::IntoRequest<super::SelectFindingRequest>,
        ) -> std::result::Result<
            tonic::Response<super::SelectFindingResponse>,
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
                "/agni.v1.viewer.ViewerPageService/SelectFinding",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "SelectFinding"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn locate_entity(
            &mut self,
            request: impl tonic::IntoRequest<super::LocateEntityRequest>,
        ) -> std::result::Result<
            tonic::Response<super::LocateEntityResponse>,
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
                "/agni.v1.viewer.ViewerPageService/LocateEntity",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "LocateEntity"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn show_sheet(
            &mut self,
            request: impl tonic::IntoRequest<super::ShowSheetRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ShowSheetResponse>,
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
                "/agni.v1.viewer.ViewerPageService/ShowSheet",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "ShowSheet"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn open_sheet_view(
            &mut self,
            request: impl tonic::IntoRequest<super::OpenSheetViewRequest>,
        ) -> std::result::Result<
            tonic::Response<super::OpenSheetViewResponse>,
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
                "/agni.v1.viewer.ViewerPageService/OpenSheetView",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "OpenSheetView"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn close_sheet_view(
            &mut self,
            request: impl tonic::IntoRequest<super::CloseSheetViewRequest>,
        ) -> std::result::Result<
            tonic::Response<super::CloseSheetViewResponse>,
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
                "/agni.v1.viewer.ViewerPageService/CloseSheetView",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "CloseSheetView"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn show_answer(
            &mut self,
            request: impl tonic::IntoRequest<super::ShowAnswerRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ShowAnswerResponse>,
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
                "/agni.v1.viewer.ViewerPageService/ShowAnswer",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(
                    GrpcMethod::new("agni.v1.viewer.ViewerPageService", "ShowAnswer"),
                );
            self.inner.unary(req, path, codec).await
        }
        pub async fn propose(
            &mut self,
            request: impl tonic::IntoRequest<super::ProposeRequest>,
        ) -> std::result::Result<
            tonic::Response<super::ProposeResponse>,
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
                "/agni.v1.viewer.ViewerPageService/Propose",
            );
            let mut req = request.into_request();
            req.extensions_mut()
                .insert(GrpcMethod::new("agni.v1.viewer.ViewerPageService", "Propose"));
            self.inner.unary(req, path, codec).await
        }
    }
}
