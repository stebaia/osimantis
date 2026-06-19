import 'package:equatable/equatable.dart';

/// Failure è l'errore di dominio mostrabile all'utente. Il layer data converte
/// le eccezioni tecniche (Dio, parsing) in una di queste.
sealed class Failure extends Equatable {
  const Failure(this.message);
  final String message;

  @override
  List<Object?> get props => [message];
}

/// Problema di rete/connessione (timeout, host irraggiungibile).
class NetworkFailure extends Failure {
  const NetworkFailure([super.message = 'Errore di connessione']);
}

/// Token mancante o non valido (401).
class AuthFailure extends Failure {
  const AuthFailure([super.message = 'Autenticazione richiesta']);
}

/// Errore lato server (5xx) o risposta inattesa.
class ServerFailure extends Failure {
  const ServerFailure([super.message = 'Errore del server']);
}
