import 'failure.dart';

/// Result minimale (Either-like) senza dipendenze esterne: o un valore [data]
/// in caso di successo, o una [Failure]. Le usecase/repository lo restituiscono
/// così il bloc gestisce successo ed errore senza try/catch sparsi.
sealed class Result<T> {
  const Result();
}

class Success<T> extends Result<T> {
  const Success(this.data);
  final T data;
}

class Error<T> extends Result<T> {
  const Error(this.failure);
  final Failure failure;
}
