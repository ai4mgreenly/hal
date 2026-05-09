# R-3JCR-C810: Dynamic Client Registration endpoint (RFC 7591). MCP
# clients POST a JSON registration request; the service mints a
# client_id (and a client_secret for confidential clients) and replies
# with the registration response document.
class OauthRegisterController < ActionController::API
  def create
    params = registration_params
    record, secret_plain = OauthClient.register(params)

    body = {
      client_id: record.client_id,
      client_id_issued_at: record.created_at.to_i,
      redirect_uris: record.redirect_uris,
      grant_types: record.grant_types,
      response_types: record.response_types,
      token_endpoint_auth_method: record.token_endpoint_auth_method
    }
    body[:client_name] = record.client_name if record.client_name.present?
    body[:client_secret] = secret_plain if secret_plain
    render json: body, status: :created
  rescue ArgumentError, ActiveRecord::RecordInvalid => e
    render json: { error: "invalid_client_metadata", error_description: e.message },
           status: :bad_request
  end

  private

  def registration_params
    raw = request.body.read
    return {} if raw.blank?
    JSON.parse(raw)
  rescue JSON::ParserError
    {}
  end
end
